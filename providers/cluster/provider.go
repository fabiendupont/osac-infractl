// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
	"github.com/fabiendupont/infractl/workflow"
	"github.com/osac-project/osac-infractl/handlers"
)

const (
	ResourceType = "Cluster"
	Collection   = "osac.kubernetes_hcp"
)

type ClusterProvider struct {
	crud   *handlers.CRUDHandler[Cluster]
	logger zerolog.Logger
}

func New() *ClusterProvider { return &ClusterProvider{} }

func (p *ClusterProvider) Name() string           { return "cluster" }
func (p *ClusterProvider) Version() string        { return "0.1.0" }
func (p *ClusterProvider) Features() []string     { return []string{"cluster"} }
func (p *ClusterProvider) Dependencies() []string { return []string{"hosttype"} }

func (p *ClusterProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[Cluster](ctx.DB)
	p.crud = handlers.NewCRUDHandler[Cluster](store, func() *Cluster {
		return &Cluster{Status: resource.JSONField[ClusterStatus]{Data: ClusterStatus{Phase: "Progressing"}}}
	})
	if ctx.Hooks != nil {
		p.crud.WithHooks(ctx.Hooks, ResourceType)
	}
	if err := ctx.DB.AutoMigrate(&Cluster{}); err != nil {
		return err
	}

	// Validate that all referenced HostTypes exist.
	if ctx.Registry != nil {
		db := ctx.DB
		ctx.Registry.RegisterHook(provider.SyncHook{
			Feature: ResourceType,
			Event:   "pre_create",
			Handler: validateClusterHostTypes(db),
		})
	}

	p.logger.Info().Msg("cluster provider initialized")
	return nil
}

func validateClusterHostTypes(db *gorm.DB) func(context.Context, interface{}) error {
	return func(ctx context.Context, payload interface{}) error {
		c, ok := payload.(*Cluster)
		if !ok {
			return nil
		}
		for _, ns := range c.Spec.Data.NodeSets {
			if ns.HostType == "" {
				continue
			}
			var count int64
			if err := db.WithContext(ctx).
				Table("host_types").
				Where("org_id = ? AND name = ? AND deleted_at IS NULL", c.OrgID, ns.HostType).
				Count(&count).Error; err != nil {
				return fmt.Errorf("validating node_sets.host_type: %w", err)
			}
			if count == 0 {
				return fmt.Errorf("HostType %q not found (referenced by node_sets[%s])", ns.HostType, ns.Name)
			}
		}
		return nil
	}
}

func (p *ClusterProvider) Shutdown(_ context.Context) error { return nil }

func (p *ClusterProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/clusters")
}

func (p *ClusterProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType,
		Event:        "create",
		Phase:        workflow.PhaseMain,
		Priority:     100,
		Ref:          Collection + "-create",
		Metadata:     map[string]string{"collection": Collection, "resource_action": "cluster"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType,
		Event:        "delete",
		Phase:        workflow.PhaseMain,
		Priority:     100,
		Ref:          Collection + "-delete",
		Metadata:     map[string]string{"collection": Collection, "resource_action": "cluster"},
	})
}

var _ provider.Provider = (*ClusterProvider)(nil)
var _ provider.APIProvider = (*ClusterProvider)(nil)
var _ provider.WorkflowProvider = (*ClusterProvider)(nil)
