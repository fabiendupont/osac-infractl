// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
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

	aap "github.com/fabiendupont/infractl-executor-aap"

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

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	dsn := os.Getenv("OSAC_DB_DSN")
	if dsn == "" {
		logger.Fatal().Msg("OSAC_DB_DSN is required")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}

	// --- AAP Executor ---
	executor, err := setupAAP(logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to configure AAP executor")
	}

	dispatchTable := workflow.NewDispatchTable()

	// --- Provider Registry ---
	registry := provider.NewRegistry()

	for _, p := range []provider.Provider{
		tenant.New(),
		event.New(),
		secret.New(),
		task.New(),
		webhook.New(),
		policy.New(),
	} {
		if err := registry.Register(p); err != nil {
			logger.Fatal().Err(err).Msg("failed to register platform provider")
		}
	}

	for _, p := range []provider.Provider{
		hosttype.New(),
		networkclass.New(),
		project.New(),
		virtualnetwork.New(),
		subnet.New(),
		securitygroup.New(),
		publicippool.New(),
		publicip.New(),
		cluster.New(),
		computeinstance.New(),
	} {
		if err := registry.Register(p); err != nil {
			logger.Fatal().Err(err).Msg("failed to register OSAC provider")
		}
	}

	if err := registry.ResolveDependencies(); err != nil {
		logger.Fatal().Err(err).Msg("failed to resolve provider dependencies")
	}

	bus, err := events.NewPgBus(db, dsn, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create event bus")
	}

	queue, err := work.NewPgQueue(db, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create task queue")
	}

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
		Executor:      executor,
	}

	if err := registry.InitAll(provCtx); err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize providers")
	}

	// Let WorkflowProviders register their actions.
	for _, wp := range registry.WorkflowProviders() {
		wp.RegisterActions(dispatchTable)
	}

	// Create the dispatcher and register hooks so lifecycle events
	// trigger workflow execution.
	dispatcher := workflow.NewDispatcher(dispatchTable, executor, logger)
	dispatcher.RegisterHooks(registry)

	// --- HTTP Server ---
	addr := os.Getenv("OSAC_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := api.NewServer(api.ServerConfig{Addr: addr}, logger)

	srv.Router.Group(func(r chi.Router) {
		r.Use(auth.AuthN(&auth.GuestAuthenticator{}))
		r.Use(auth.Tenancy(&auth.GuestTenancyLogic{
			DefaultTenant: "00000000-0000-0000-0000-000000000001",
		}))
		r.Use(auth.AuthZ(&auth.AllowAllAuthorizer{}))

		r.Route("/api/v1", func(r chi.Router) {
			for _, ap := range registry.APIProviders() {
				ap.RegisterRoutes(r)
			}
		})
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	bus.StartCleanup(ctx, 7*24*time.Hour, 1*time.Hour)
	queue.StartRecovery(ctx, 15*time.Minute, 1*time.Minute)

	httpSrv := srv.HTTPServer()
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server error")
		}
	}()

	logger.Info().
		Int("platform_providers", 6).
		Int("domain_providers", 10).
		Int("dispatch_actions", len(dispatchTable.ResourceTypes())).
		Msg("OSAC server started")

	<-ctx.Done()
	logger.Info().Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Drain the executor before shutting down the server.
	if aapExec, ok := executor.(*aap.Executor); ok {
		if err := aapExec.Drain(shutdownCtx); err != nil {
			logger.Warn().Err(err).Msg("executor drain incomplete")
		}
	}

	httpSrv.Shutdown(shutdownCtx)
	registry.ShutdownAll(shutdownCtx)
	bus.Close()
}

func setupAAP(logger zerolog.Logger) (workflow.Executor, error) {
	baseURL := os.Getenv("AAP_BASE_URL")
	if baseURL == "" {
		logger.Info().Msg("AAP_BASE_URL not set, using local executor")
		return workflow.NewLocalExecutor(), nil
	}

	var authMethod aap.AuthMethod

	if tokenURL := os.Getenv("AAP_OAUTH2_TOKEN_URL"); tokenURL != "" {
		authMethod = aap.NewOAuth2Auth(aap.OAuth2Config{
			TokenURL:     tokenURL,
			ClientID:     os.Getenv("AAP_OAUTH2_CLIENT_ID"),
			ClientSecret: os.Getenv("AAP_OAUTH2_CLIENT_SECRET"),
		}, http.DefaultClient)
	} else if token := os.Getenv("AAP_TOKEN"); token != "" {
		authMethod = &aap.BearerTokenAuth{Token: token}
	}

	retryCfg := aap.DefaultRetryConfig()
	circuitCfg := aap.DefaultCircuitConfig()

	client, err := aap.NewClient(aap.ClientConfig{
		BaseURL: baseURL,
		Auth:    authMethod,
		TLS: aap.TLSConfig{
			CACert:     os.Getenv("AAP_CA_CERT"),
			ClientCert: os.Getenv("AAP_CLIENT_CERT"),
			ClientKey:  os.Getenv("AAP_CLIENT_KEY"),
			Insecure:   os.Getenv("AAP_INSECURE") == "true",
		},
		Retry:   &retryCfg,
		Circuit: &circuitCfg,
	})
	if err != nil {
		return nil, err
	}

	maxInFlight := 10
	if v := os.Getenv("AAP_MAX_IN_FLIGHT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxInFlight = n
		}
	}

	return aap.NewExecutor(client, logger, aap.ExecutorConfig{
		MaxInFlight: maxInFlight,
	}), nil
}
