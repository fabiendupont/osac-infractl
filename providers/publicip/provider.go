// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package publicip

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
	"github.com/fabiendupont/infractl/workflow"
	"github.com/osac-project/osac-infractl/handlers"
)

const (
	ResourceType = "PublicIP"
	Collection   = "osac.networking_metallb"
)

type PublicIPProvider struct {
	crud   *handlers.CRUDHandler[PublicIP]
	logger zerolog.Logger
}

func New() *PublicIPProvider { return &PublicIPProvider{} }
func (p *PublicIPProvider) Name() string           { return "publicip" }
func (p *PublicIPProvider) Version() string        { return "0.1.0" }
func (p *PublicIPProvider) Features() []string     { return []string{"publicip"} }
func (p *PublicIPProvider) Dependencies() []string { return []string{"publicippool"} }

func (p *PublicIPProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[PublicIP](ctx.DB)
	p.crud = handlers.NewCRUDHandler[PublicIP](store, func() *PublicIP {
		return &PublicIP{Status: resource.JSONField[PublicIPStatus]{Data: PublicIPStatus{Phase: "Pending"}}}
	})
	if ctx.Hooks != nil {
		p.crud.WithHooks(ctx.Hooks, ResourceType)
	}
	if err := ctx.DB.AutoMigrate(&PublicIP{}); err != nil {
		return err
	}

	if ctx.Registry != nil {
		ctx.Registry.RegisterRef(provider.ResourceRef{
			Source: ResourceType, Field: "spec.pool",
			Target: "PublicIPPool", Table: "public_ip_pools", Required: true,
			Extract: func(payload interface{}) (uuid.UUID, string) {
				if ip, ok := payload.(*PublicIP); ok {
					return ip.OrgID, ip.Spec.Data.Pool
				}
				return uuid.Nil, ""
			},
		}, ctx.DB)
	}

	p.logger.Info().Msg("public IP provider initialized")
	return nil
}

func (p *PublicIPProvider) Shutdown(_ context.Context) error { return nil }
func (p *PublicIPProvider) RegisterRoutes(r chi.Router) { p.crud.RegisterRoutes(r, "/public-ips") }

func (p *PublicIPProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "create", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-public-ip-create", Metadata: map[string]string{"collection": Collection, "resource_action": "public_ip"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "delete", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-public-ip-delete", Metadata: map[string]string{"collection": Collection, "resource_action": "public_ip"},
	})
}

var _ provider.Provider = (*PublicIPProvider)(nil)
var _ provider.APIProvider = (*PublicIPProvider)(nil)
var _ provider.WorkflowProvider = (*PublicIPProvider)(nil)
