// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupKeycloak starts a Keycloak testcontainer in dev mode and returns
// the base URL and a cleanup function.
func SetupKeycloak(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "quay.io/keycloak/keycloak:26.2",
			ExposedPorts: []string{"8080/tcp"},
			Cmd:          []string{"start-dev"},
			Env: map[string]string{
				"KC_BOOTSTRAP_ADMIN_USERNAME": "admin",
				"KC_BOOTSTRAP_ADMIN_PASSWORD": "admin",
			},
			WaitingFor: wait.ForHTTP("/realms/master").
				WithPort("8080/tcp").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to start keycloak container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get keycloak host: %v", err)
	}

	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get keycloak port: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	cleanup := func() {
		container.Terminate(ctx)
	}

	return baseURL, cleanup
}
