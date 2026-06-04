// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/workflow"
)

// DiscoveryConfig holds configuration for the discovery service.
type DiscoveryConfig struct {
	RefreshInterval time.Duration
}

// Discovery queries Automation Hub for OSAC add-on collections and
// populates the workflow dispatch table from their metadata.
type Discovery struct {
	client *Client
	table  *workflow.DispatchTable
	config DiscoveryConfig
	logger zerolog.Logger

	mu        sync.RWMutex
	manifests map[string]*AddonManifest // collection FQCN → manifest
	actions   int
}

// NewDiscovery creates a discovery service.
func NewDiscovery(client *Client, table *workflow.DispatchTable, cfg DiscoveryConfig, logger zerolog.Logger) *Discovery {
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = 5 * time.Minute
	}
	return &Discovery{
		client:    client,
		table:     table,
		config:    cfg,
		logger:    logger.With().Str("component", "hub-discovery").Logger(),
		manifests: make(map[string]*AddonManifest),
	}
}

// Discover performs a one-shot discovery: lists collections, fetches
// metadata, and registers handlers in the dispatch table. Returns the
// number of actions discovered.
func (d *Discovery) Discover(ctx context.Context) (int, error) {
	collections, err := d.client.ListCollections(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing collections: %w", err)
	}

	d.logger.Info().Int("collections", len(collections)).Msg("collections found")

	actions := 0
	for _, col := range collections {
		fqcn := col.Namespace + "." + col.Name
		version := col.Version
		if version == "" {
			continue
		}

		detail, err := d.client.GetCollectionVersion(ctx, col.Namespace, col.Name, version)
		if err != nil {
			d.logger.Warn().Err(err).Str("collection", fqcn).Msg("failed to get collection detail")
			continue
		}

		manifest, roles, err := d.parseCollectionMetadata(detail)
		if err != nil {
			d.logger.Warn().Err(err).Str("collection", fqcn).Msg("failed to parse collection metadata")
			continue
		}

		if manifest != nil {
			d.mu.Lock()
			d.manifests[fqcn] = manifest
			d.mu.Unlock()
		}

		for roleName, role := range roles {
			handler := role.ToHandler(fqcn, roleName)
			d.table.Register(handler)
			actions++
			d.logger.Debug().
				Str("collection", fqcn).
				Str("role", roleName).
				Str("resource_type", role.ResourceType).
				Str("event", role.Event).
				Msg("registered action")
		}
	}

	d.mu.Lock()
	d.actions = actions
	d.mu.Unlock()

	d.logger.Info().
		Int("actions", actions).
		Int("collections", len(collections)).
		Msg("discovery complete")

	return actions, nil
}

// StartRefresh begins periodic re-discovery. Runs until context is cancelled.
func (d *Discovery) StartRefresh(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(d.config.RefreshInterval)
		defer ticker.Stop()

		d.logger.Info().Dur("interval", d.config.RefreshInterval).Msg("periodic refresh started")

		for {
			select {
			case <-ctx.Done():
				d.logger.Info().Msg("periodic refresh stopped")
				return
			case <-ticker.C:
				if _, err := d.Discover(ctx); err != nil {
					d.logger.Error().Err(err).Msg("periodic discovery failed")
				}
			}
		}
	}()
}

// ActionCount returns the number of actions currently registered from
// Hub discovery.
func (d *Discovery) ActionCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.actions
}

// Manifests returns all discovered add-on manifests keyed by FQCN.
func (d *Discovery) Manifests() map[string]*AddonManifest {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make(map[string]*AddonManifest, len(d.manifests))
	for k, v := range d.manifests {
		result[k] = v
	}
	return result
}

// parseCollectionMetadata extracts addon.yaml and role osac.yaml from
// the collection's metadata JSON. The Hub API returns metadata as a
// JSON blob that contains the collection's files info.
func (d *Discovery) parseCollectionMetadata(detail *CollectionDetail) (*AddonManifest, map[string]*RoleMetadata, error) {
	if detail.Metadata == nil {
		return nil, nil, nil
	}

	// The Hub API metadata contains the collection's contents description.
	// We look for addon manifest and role metadata embedded in it.
	var meta struct {
		Contents []struct {
			Name        string `json:"name"`
			ContentType string `json:"content_type"`
			Description string `json:"description"`
		} `json:"contents"`
		Tags     []string        `json:"tags"`
		AddonYAML json.RawMessage `json:"addon_yaml,omitempty"`
		Roles     map[string]struct {
			OsacYAML json.RawMessage `json:"osac_yaml,omitempty"`
		} `json:"roles,omitempty"`
	}

	if err := json.Unmarshal(detail.Metadata, &meta); err != nil {
		// Metadata format may vary; treat parse failure as non-fatal.
		return nil, nil, nil
	}

	var manifest *AddonManifest
	if meta.AddonYAML != nil {
		manifest = &AddonManifest{}
		if err := json.Unmarshal(meta.AddonYAML, manifest); err != nil {
			d.logger.Warn().Err(err).Msg("failed to parse addon_yaml")
		}
	}

	roles := make(map[string]*RoleMetadata)
	for roleName, roleData := range meta.Roles {
		if roleData.OsacYAML == nil {
			continue
		}
		role := &RoleMetadata{}
		if err := json.Unmarshal(roleData.OsacYAML, role); err != nil {
			d.logger.Warn().Err(err).Str("role", roleName).Msg("failed to parse osac_yaml")
			continue
		}
		roles[roleName] = role
	}

	return manifest, roles, nil
}
