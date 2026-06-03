// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package networkclass

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
	"github.com/osac-project/osac-infractl/handlers"
)

type NetworkClassProvider struct {
	crud   *handlers.CRUDHandler[NetworkClass]
	logger zerolog.Logger
}

func New() *NetworkClassProvider { return &NetworkClassProvider{} }

func (p *NetworkClassProvider) Name() string           { return "networkclass" }
func (p *NetworkClassProvider) Version() string        { return "0.1.0" }
func (p *NetworkClassProvider) Features() []string     { return []string{"networkclass"} }
func (p *NetworkClassProvider) Dependencies() []string { return nil }

func (p *NetworkClassProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[NetworkClass](ctx.DB)
	p.crud = handlers.NewCRUDHandler[NetworkClass](store, func() *NetworkClass {
		return &NetworkClass{Status: resource.JSONField[NetworkClassStatus]{Data: NetworkClassStatus{Phase: "Active"}}}
	})
	if err := ctx.DB.AutoMigrate(&NetworkClass{}); err != nil {
		return err
	}
	p.logger.Info().Msg("network class provider initialized")
	return nil
}

func (p *NetworkClassProvider) Shutdown(_ context.Context) error { return nil }

func (p *NetworkClassProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/network-classes")
}

var _ provider.Provider = (*NetworkClassProvider)(nil)
var _ provider.APIProvider = (*NetworkClassProvider)(nil)
