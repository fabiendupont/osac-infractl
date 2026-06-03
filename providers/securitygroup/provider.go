// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package securitygroup

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
	ResourceType = "SecurityGroup"
	Collection   = "osac.networking_ovnk"
)

type SecurityGroupProvider struct {
	crud   *handlers.CRUDHandler[SecurityGroup]
	logger zerolog.Logger
}

func New() *SecurityGroupProvider { return &SecurityGroupProvider{} }

func (p *SecurityGroupProvider) Name() string           { return "securitygroup" }
func (p *SecurityGroupProvider) Version() string        { return "0.1.0" }
func (p *SecurityGroupProvider) Features() []string     { return []string{"securitygroup"} }
func (p *SecurityGroupProvider) Dependencies() []string { return []string{"virtualnetwork"} }

func (p *SecurityGroupProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[SecurityGroup](ctx.DB)
	p.crud = handlers.NewCRUDHandler[SecurityGroup](store, func() *SecurityGroup {
		return &SecurityGroup{Status: resource.JSONField[SecurityGroupStatus]{Data: SecurityGroupStatus{Phase: "Pending"}}}
	})
	if ctx.Hooks != nil {
		p.crud.WithHooks(ctx.Hooks, ResourceType)
	}
	if err := ctx.DB.AutoMigrate(&SecurityGroup{}); err != nil {
		return err
	}
	p.logger.Info().Msg("security group provider initialized")
	return nil
}

func (p *SecurityGroupProvider) Shutdown(_ context.Context) error { return nil }

func (p *SecurityGroupProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/security-groups")
}

func (p *SecurityGroupProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "create", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-security-group-create", Metadata: map[string]string{"collection": Collection, "resource_action": "security_group"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "delete", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-security-group-delete", Metadata: map[string]string{"collection": Collection, "resource_action": "security_group"},
	})
}

var _ provider.Provider = (*SecurityGroupProvider)(nil)
var _ provider.APIProvider = (*SecurityGroupProvider)(nil)
var _ provider.WorkflowProvider = (*SecurityGroupProvider)(nil)
