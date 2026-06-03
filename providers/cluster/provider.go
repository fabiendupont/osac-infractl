// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package cluster

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
	if err := ctx.DB.AutoMigrate(&Cluster{}); err != nil {
		return err
	}
	p.logger.Info().Msg("cluster provider initialized")
	return nil
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
