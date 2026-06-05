// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package blueprint

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
	"github.com/osac-project/osac-infractl/handlers"
)

type BlueprintProvider struct {
	crud   *handlers.CRUDHandler[Blueprint]
	logger zerolog.Logger
}

func New() *BlueprintProvider { return &BlueprintProvider{} }

func (p *BlueprintProvider) Name() string           { return "blueprint" }
func (p *BlueprintProvider) Version() string        { return "0.1.0" }
func (p *BlueprintProvider) Features() []string     { return []string{"blueprint"} }
func (p *BlueprintProvider) Dependencies() []string { return []string{"catalog"} }

func (p *BlueprintProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[Blueprint](ctx.DB)
	p.crud = handlers.NewCRUDHandler[Blueprint](store, func() *Blueprint {
		return &Blueprint{Status: resource.JSONField[BlueprintStatus]{Data: BlueprintStatus{Phase: "Pending"}}}
	})
	if ctx.Hooks != nil {
		p.crud.WithHooks(ctx.Hooks, "Blueprint")
	}
	if err := ctx.DB.AutoMigrate(&Blueprint{}); err != nil {
		return err
	}
	p.logger.Info().Msg("blueprint provider initialized")
	return nil
}

func (p *BlueprintProvider) Shutdown(_ context.Context) error { return nil }

func (p *BlueprintProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/blueprints")
}

var _ provider.Provider = (*BlueprintProvider)(nil)
var _ provider.APIProvider = (*BlueprintProvider)(nil)
