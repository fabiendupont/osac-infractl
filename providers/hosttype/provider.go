// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package hosttype

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
	"github.com/osac-project/osac-infractl/handlers"
)

type HostTypeProvider struct {
	crud *handlers.CRUDHandler[HostType]
	logger zerolog.Logger
}

func New() *HostTypeProvider { return &HostTypeProvider{} }

func (p *HostTypeProvider) Name() string           { return "hosttype" }
func (p *HostTypeProvider) Version() string        { return "0.1.0" }
func (p *HostTypeProvider) Features() []string     { return []string{"hosttype"} }
func (p *HostTypeProvider) Dependencies() []string { return nil }

func (p *HostTypeProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[HostType](ctx.DB)
	p.crud = handlers.NewCRUDHandler[HostType](store, func() *HostType {
		return &HostType{Status: resource.JSONField[HostTypeStatus]{Data: HostTypeStatus{Phase: "Active"}}}
	})
	if err := ctx.DB.AutoMigrate(&HostType{}); err != nil {
		return err
	}
	p.logger.Info().Msg("host type provider initialized")
	return nil
}

func (p *HostTypeProvider) Shutdown(_ context.Context) error { return nil }

func (p *HostTypeProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/host-types")
}

var _ provider.Provider = (*HostTypeProvider)(nil)
var _ provider.APIProvider = (*HostTypeProvider)(nil)
