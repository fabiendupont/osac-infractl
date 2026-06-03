// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package subnet

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
	ResourceType = "Subnet"
	Collection   = "osac.networking_ovnk"
)

type SubnetProvider struct {
	crud   *handlers.CRUDHandler[Subnet]
	logger zerolog.Logger
}

func New() *SubnetProvider { return &SubnetProvider{} }

func (p *SubnetProvider) Name() string           { return "subnet" }
func (p *SubnetProvider) Version() string        { return "0.1.0" }
func (p *SubnetProvider) Features() []string     { return []string{"subnet"} }
func (p *SubnetProvider) Dependencies() []string { return []string{"virtualnetwork"} }

func (p *SubnetProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[Subnet](ctx.DB)
	p.crud = handlers.NewCRUDHandler[Subnet](store, func() *Subnet {
		return &Subnet{Status: resource.JSONField[SubnetStatus]{Data: SubnetStatus{Phase: "Pending"}}}
	})
	if err := ctx.DB.AutoMigrate(&Subnet{}); err != nil {
		return err
	}
	p.logger.Info().Msg("subnet provider initialized")
	return nil
}

func (p *SubnetProvider) Shutdown(_ context.Context) error { return nil }

func (p *SubnetProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/subnets")
}

func (p *SubnetProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "create", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-subnet-create", Metadata: map[string]string{"collection": Collection, "resource_action": "subnet"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "delete", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-subnet-delete", Metadata: map[string]string{"collection": Collection, "resource_action": "subnet"},
	})
}

var _ provider.Provider = (*SubnetProvider)(nil)
var _ provider.APIProvider = (*SubnetProvider)(nil)
var _ provider.WorkflowProvider = (*SubnetProvider)(nil)
