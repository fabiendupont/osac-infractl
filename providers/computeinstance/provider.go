// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package computeinstance

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
	ResourceType = "ComputeInstance"
	Collection   = "osac.compute_kubevirt"
)

type ComputeInstanceProvider struct {
	crud   *handlers.CRUDHandler[ComputeInstance]
	logger zerolog.Logger
}

func New() *ComputeInstanceProvider { return &ComputeInstanceProvider{} }

func (p *ComputeInstanceProvider) Name() string           { return "computeinstance" }
func (p *ComputeInstanceProvider) Version() string        { return "0.1.0" }
func (p *ComputeInstanceProvider) Features() []string     { return []string{"computeinstance"} }
func (p *ComputeInstanceProvider) Dependencies() []string { return []string{"subnet", "securitygroup"} }

func (p *ComputeInstanceProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[ComputeInstance](ctx.DB)
	p.crud = handlers.NewCRUDHandler[ComputeInstance](store, func() *ComputeInstance {
		return &ComputeInstance{Status: resource.JSONField[ComputeInstanceStatus]{Data: ComputeInstanceStatus{Phase: "Starting"}}}
	})
	if err := ctx.DB.AutoMigrate(&ComputeInstance{}); err != nil {
		return err
	}
	p.logger.Info().Msg("compute instance provider initialized")
	return nil
}

func (p *ComputeInstanceProvider) Shutdown(_ context.Context) error { return nil }

func (p *ComputeInstanceProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/compute-instances")
}

func (p *ComputeInstanceProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType,
		Event:        "create",
		Phase:        workflow.PhaseMain,
		Priority:     100,
		Ref:          Collection + "-create",
		Metadata:     map[string]string{"collection": Collection, "resource_action": "instance"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType,
		Event:        "delete",
		Phase:        workflow.PhaseMain,
		Priority:     100,
		Ref:          Collection + "-delete",
		Metadata:     map[string]string{"collection": Collection, "resource_action": "instance"},
	})
}

var _ provider.Provider = (*ComputeInstanceProvider)(nil)
var _ provider.APIProvider = (*ComputeInstanceProvider)(nil)
var _ provider.WorkflowProvider = (*ComputeInstanceProvider)(nil)
