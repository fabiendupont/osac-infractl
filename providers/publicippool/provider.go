// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package publicippool

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
	"github.com/fabiendupont/infractl/workflow"
	"github.com/osac-project/osac-infractl/handlers"
)

const (
	ResourceType = "PublicIPPool"
	Collection   = "osac.networking_metallb"
)

type PublicIPPoolProvider struct {
	crud   *handlers.CRUDHandler[PublicIPPool]
	logger zerolog.Logger
}

func New() *PublicIPPoolProvider { return &PublicIPPoolProvider{} }

func (p *PublicIPPoolProvider) Name() string           { return "publicippool" }
func (p *PublicIPPoolProvider) Version() string        { return "0.1.0" }
func (p *PublicIPPoolProvider) Features() []string     { return []string{"publicippool"} }
func (p *PublicIPPoolProvider) Dependencies() []string { return nil }

func (p *PublicIPPoolProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[PublicIPPool](ctx.DB)
	p.crud = handlers.NewCRUDHandler[PublicIPPool](store, func() *PublicIPPool {
		return &PublicIPPool{Status: resource.JSONField[PublicIPPoolStatus]{Data: PublicIPPoolStatus{Phase: "Pending"}}}
	})
	if ctx.Hooks != nil {
		p.crud.WithHooks(ctx.Hooks, ResourceType)
	}
	if err := ctx.DB.AutoMigrate(&PublicIPPool{}); err != nil {
		return err
	}
	p.logger.Info().Msg("public IP pool provider initialized")
	return nil
}

func (p *PublicIPPoolProvider) Shutdown(_ context.Context) error { return nil }

func (p *PublicIPPoolProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/public-ip-pools")
}

func (p *PublicIPPoolProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "create", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-public-ip-pool-create", Metadata: map[string]string{"collection": Collection, "resource_action": "public_ip_pool"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "delete", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-public-ip-pool-delete", Metadata: map[string]string{"collection": Collection, "resource_action": "public_ip_pool"},
	})
}

var _ provider.Provider = (*PublicIPPoolProvider)(nil)
var _ provider.APIProvider = (*PublicIPPoolProvider)(nil)
var _ provider.WorkflowProvider = (*PublicIPPoolProvider)(nil)
