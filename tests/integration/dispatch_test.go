// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	aap "github.com/fabiendupont/infractl-executor-aap"
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
	"github.com/fabiendupont/infractl/resource"
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
	"github.com/osac-project/osac-infractl/tests/testutil"
)

const defaultTenant = "00000000-0000-0000-0000-000000000001"

// TestFullDispatchLoop validates the entire lifecycle:
// API request → Store → Hook → Dispatch → AAP executor → Poll → Status update
func TestFullDispatchLoop(t *testing.T) {
	// Start PostgreSQL.
	ctx := context.Background()
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("osac_dispatch_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Start mock AAP — jobs complete after 2 polls.
	mockAAP := testutil.NewMockAAP(t, 2)
	defer mockAAP.Close()

	logger := zerolog.Nop()

	// Create AAP executor pointing at mock.
	aapClient, err := aap.NewClient(aap.ClientConfig{
		BaseURL: mockAAP.URL(),
		Auth:    &aap.BearerTokenAuth{Token: "test"},
	})
	require.NoError(t, err)
	executor := aap.NewExecutor(aapClient, logger, aap.ExecutorConfig{MaxInFlight: 10})

	dispatchTable := workflow.NewDispatchTable()

	// Register all providers.
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
		Executor:      executor,
	}
	require.NoError(t, registry.InitAll(provCtx))

	for _, wp := range registry.WorkflowProviders() {
		wp.RegisterActions(dispatchTable)
	}

	dispatcher := workflow.NewDispatcher(dispatchTable, executor, logger)
	dispatcher.RegisterHooks(registry)

	// Status polling with store registry.
	storeRegistry := workflow.NewStoreRegistry()
	storeRegistry.Register("Cluster", resource.NewStatusUpdater(db, "clusters"))
	statusCallback := workflow.MakeStatusCallback(storeRegistry)

	testCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher.StartPolling(testCtx, 200*time.Millisecond, statusCallback)

	// Build HTTP server.
	srv := api.NewServer(api.ServerConfig{Addr: ":0"}, logger)
	srv.Router.Group(func(r chi.Router) {
		r.Use(auth.AuthN(&auth.GuestAuthenticator{}))
		r.Use(auth.Tenancy(&auth.GuestTenancyLogic{DefaultTenant: defaultTenant}))
		r.Use(auth.AuthZ(&auth.AllowAllAuthorizer{}))
		r.Route("/api/v1", func(r chi.Router) {
			for _, ap := range registry.APIProviders() {
				ap.RegisterRoutes(r)
			}
		})
	})
	ts := httptest.NewServer(srv.Router)
	defer ts.Close()

	// --- Test: Create a host type (dependency for cluster) ---
	resp := doPost(t, ts.URL+"/api/v1/host-types", map[string]interface{}{
		"name": "standard",
		"spec": map[string]interface{}{"Data": map[string]interface{}{"title": "Standard"}},
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// --- Test: Create a cluster (triggers dispatch to mock AAP) ---
	resp = doPost(t, ts.URL+"/api/v1/clusters", map[string]interface{}{
		"name": "dispatch-test",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"release_image": "quay.io/ocp:4.16",
		}},
	})
	body := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(body))

	// Give the async hook + dispatch + polling time to complete.
	// Mock AAP completes after 2 polls at 200ms interval = ~400ms.
	time.Sleep(2 * time.Second)

	// Verify mock AAP received the job launch.
	assert.Equal(t, 1, mockAAP.JobCount(), "expected 1 AAP job to be launched")

	// Verify the cluster status was updated to Ready.
	var statusJSON string
	err = db.Table("clusters").
		Where("name = ? AND deleted_at IS NULL", "dispatch-test").
		Pluck("status", &statusJSON).Error
	require.NoError(t, err)

	var status map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(statusJSON), &status))
	assert.Equal(t, "Ready", status["phase"], "cluster status: %s", statusJSON)
}

func doPost(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-ID", defaultTenant)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b
}
