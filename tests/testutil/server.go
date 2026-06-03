// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fabiendupont/infractl/api"
	"github.com/fabiendupont/infractl/auth"
	"github.com/fabiendupont/infractl/events"
	"github.com/fabiendupont/infractl/platform/event"
	"github.com/fabiendupont/infractl/platform/policy"
	"github.com/fabiendupont/infractl/platform/secret"
	"github.com/fabiendupont/infractl/platform/task"
	"github.com/fabiendupont/infractl/platform/tenant"
	"github.com/fabiendupont/infractl/platform/webhook"
	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/work"
	"github.com/fabiendupont/infractl/workflow"

	"github.com/osac-project/osac-infractl/providers/cluster"
	"github.com/osac-project/osac-infractl/providers/computeinstance"
	"github.com/osac-project/osac-infractl/providers/hosttype"
	"github.com/osac-project/osac-infractl/providers/networkclass"
	"github.com/osac-project/osac-infractl/providers/project"
	"github.com/osac-project/osac-infractl/providers/publicip"
	"github.com/osac-project/osac-infractl/providers/publicippool"
	"github.com/osac-project/osac-infractl/providers/securitygroup"
	"github.com/osac-project/osac-infractl/providers/subnet"
	"github.com/osac-project/osac-infractl/providers/virtualnetwork"
)

const DefaultTenant = "00000000-0000-0000-0000-000000000001"

// SetupOSACServer starts a PostgreSQL container, boots the full OSAC
// server with all providers and the local executor, and returns the
// test server URL, the local executor (for inspection), and a cleanup func.
func SetupOSACServer(t *testing.T) (string, *workflow.LocalExecutor, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("osac_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	logger := zerolog.Nop()

	localExec := workflow.NewLocalExecutor()
	dispatchTable := workflow.NewDispatchTable()

	registry := provider.NewRegistry()

	for _, p := range []provider.Provider{
		tenant.New(), event.New(), secret.New(),
		task.New(), webhook.New(), policy.New(),
	} {
		require.NoError(t, registry.Register(p))
	}

	for _, p := range []provider.Provider{
		hosttype.New(), networkclass.New(), project.New(),
		virtualnetwork.New(), subnet.New(), securitygroup.New(),
		publicippool.New(), publicip.New(),
		cluster.New(), computeinstance.New(),
	} {
		require.NoError(t, registry.Register(p))
	}

	require.NoError(t, registry.ResolveDependencies())

	bus := events.NewInMemoryBus()
	queue := work.NewInMemoryQueue()

	hooks := provider.NewHookRunner(registry, logger)
	provCtx := provider.Context{
		DB:            db,
		Registry:      registry,
		Hooks:         hooks,
		Logger:        logger,
		APIPrefix:     "/api/v1",
		Bus:           bus,
		Queue:         queue,
		DispatchTable: dispatchTable,
		Executor:      localExec,
	}

	require.NoError(t, registry.InitAll(provCtx))

	for _, wp := range registry.WorkflowProviders() {
		wp.RegisterActions(dispatchTable)
	}

	dispatcher := workflow.NewDispatcher(dispatchTable, localExec, logger)
	dispatcher.RegisterHooks(registry)

	srv := api.NewServer(api.ServerConfig{Addr: ":0"}, logger)

	srv.Router.Group(func(r chi.Router) {
		r.Use(auth.AuthN(&auth.GuestAuthenticator{}))
		r.Use(auth.Tenancy(&auth.GuestTenancyLogic{
			DefaultTenant: DefaultTenant,
		}))
		r.Use(auth.AuthZ(&auth.AllowAllAuthorizer{}))

		r.Route("/api/v1", func(r chi.Router) {
			for _, ap := range registry.APIProviders() {
				ap.RegisterRoutes(r)
			}
		})
	})

	ts := httptest.NewServer(srv.Router)

	cleanup := func() {
		ts.Close()
		container.Terminate(ctx)
	}

	return ts.URL, localExec, cleanup
}
