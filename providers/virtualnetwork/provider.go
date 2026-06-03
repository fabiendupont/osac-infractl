// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package virtualnetwork

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
	ResourceType = "VirtualNetwork"
	Collection   = "osac.networking_ovnk"
)

type VirtualNetworkProvider struct {
	crud   *handlers.CRUDHandler[VirtualNetwork]
	logger zerolog.Logger
}

func New() *VirtualNetworkProvider { return &VirtualNetworkProvider{} }

func (p *VirtualNetworkProvider) Name() string           { return "virtualnetwork" }
func (p *VirtualNetworkProvider) Version() string        { return "0.1.0" }
func (p *VirtualNetworkProvider) Features() []string     { return []string{"virtualnetwork"} }
func (p *VirtualNetworkProvider) Dependencies() []string { return []string{"networkclass"} }

func (p *VirtualNetworkProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[VirtualNetwork](ctx.DB)
	p.crud = handlers.NewCRUDHandler[VirtualNetwork](store, func() *VirtualNetwork {
		return &VirtualNetwork{Status: resource.JSONField[VirtualNetworkStatus]{Data: VirtualNetworkStatus{Phase: "Pending"}}}
	})
	if ctx.Hooks != nil {
		p.crud.WithHooks(ctx.Hooks, ResourceType)
	}
	if err := ctx.DB.AutoMigrate(&VirtualNetwork{}); err != nil {
		return err
	}
	p.logger.Info().Msg("virtual network provider initialized")
	return nil
}

func (p *VirtualNetworkProvider) Shutdown(_ context.Context) error { return nil }

func (p *VirtualNetworkProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/virtual-networks")
}

func (p *VirtualNetworkProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "create", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-virtual-network-create", Metadata: map[string]string{"collection": Collection, "resource_action": "virtual_network"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "delete", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-virtual-network-delete", Metadata: map[string]string{"collection": Collection, "resource_action": "virtual_network"},
	})
}

var _ provider.Provider = (*VirtualNetworkProvider)(nil)
var _ provider.APIProvider = (*VirtualNetworkProvider)(nil)
var _ provider.WorkflowProvider = (*VirtualNetworkProvider)(nil)
