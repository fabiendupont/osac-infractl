// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabiendupont/infractl/workflow"
	"github.com/osac-project/osac-infractl/tests/testutil"
)

const defaultTenant = testutil.DefaultTenant

func doRequest(t *testing.T, method, url string, body interface{}, headers map[string]string) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reqBody)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return b
}

func registerStubHandlers(exec *workflow.LocalExecutor) {
	refs := []string{
		"osac.kubernetes_hcp-create", "osac.kubernetes_hcp-delete",
		"osac.compute_kubevirt-create", "osac.compute_kubevirt-delete",
		"osac.networking_ovnk-virtual-network-create", "osac.networking_ovnk-virtual-network-delete",
		"osac.networking_ovnk-subnet-create", "osac.networking_ovnk-subnet-delete",
		"osac.networking_ovnk-security-group-create", "osac.networking_ovnk-security-group-delete",
		"osac.networking_metallb-public-ip-pool-create", "osac.networking_metallb-public-ip-pool-delete",
		"osac.networking_metallb-public-ip-create", "osac.networking_metallb-public-ip-delete",
	}
	for _, ref := range refs {
		exec.Register(ref, func(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"status": "provisioned"}, nil
		})
	}
}

// --- Server boot ---

func TestHealthz(t *testing.T) {
	baseURL, _, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()

	resp := doRequest(t, http.MethodGet, baseURL+"/healthz", nil, nil)
	body := readBody(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "ok")
}

// --- Infrastructure classes ---

func TestCreateHostType(t *testing.T) {
	baseURL, _, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()

	headers := map[string]string{"X-Org-ID": defaultTenant}
	payload := map[string]interface{}{
		"name": "gpu-h100",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"title": "GPU H100 Node", "description": "8x H100 GPUs",
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/host-types", payload, headers)
	body := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(body))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "gpu-h100", result["name"])
}

func TestCreateNetworkClass(t *testing.T) {
	baseURL, _, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()

	headers := map[string]string{"X-Org-ID": defaultTenant}
	payload := map[string]interface{}{
		"name": "default",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"title": "Default Network", "supports_ipv4": true,
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/network-classes", payload, headers)
	body := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(body))
}

// --- Networking ---

func TestCreateVirtualNetwork(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}
	payload := map[string]interface{}{
		"name": "prod-net",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"network_class": "default",
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/virtual-networks", payload, headers)
	body := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(body))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "prod-net", result["name"])

	// Give async hook time to fire.
	time.Sleep(100 * time.Millisecond)
}

func TestCreateSubnet(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}

	// Create virtual network first (subnet depends on it).
	doRequest(t, http.MethodPost, baseURL+"/api/v1/virtual-networks", map[string]interface{}{
		"name": "test-net",
		"spec": map[string]interface{}{"Data": map[string]interface{}{"network_class": "default"}},
	}, headers)

	payload := map[string]interface{}{
		"name": "test-subnet",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"virtual_network": "test-net", "ipv4_cidr": "10.0.1.0/24",
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/subnets", payload, headers)
	body := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(body))
}

// --- Compute ---

func TestCreateCluster(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}

	// Create host type first.
	doRequest(t, http.MethodPost, baseURL+"/api/v1/host-types", map[string]interface{}{
		"name": "standard",
		"spec": map[string]interface{}{"Data": map[string]interface{}{"title": "Standard"}},
	}, headers)

	payload := map[string]interface{}{
		"name": "my-cluster",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"release_image": "quay.io/openshift-release-dev/ocp-release:4.16.0",
			"node_sets":     []interface{}{map[string]interface{}{"name": "workers", "host_type": "standard", "replicas": 3}},
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/clusters", payload, headers)
	body := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(body))

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, "my-cluster", result["name"])

	time.Sleep(100 * time.Millisecond)
}

func TestCreateAndDeleteCluster(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}

	doRequest(t, http.MethodPost, baseURL+"/api/v1/host-types", map[string]interface{}{
		"name": "standard",
		"spec": map[string]interface{}{"Data": map[string]interface{}{"title": "Standard"}},
	}, headers)

	doRequest(t, http.MethodPost, baseURL+"/api/v1/clusters", map[string]interface{}{
		"name": "delete-me",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"release_image": "quay.io/openshift-release-dev/ocp-release:4.16.0",
		}},
	}, headers)

	resp := doRequest(t, http.MethodDelete, baseURL+"/api/v1/clusters/delete-me", nil, headers)
	readBody(t, resp)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	time.Sleep(100 * time.Millisecond)

	// Verify it's gone.
	resp = doRequest(t, http.MethodGet, baseURL+"/api/v1/clusters/delete-me", nil, headers)
	readBody(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- Project nesting ---

func TestCreateNestedProject(t *testing.T) {
	baseURL, _, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()

	headers := map[string]string{"X-Org-ID": defaultTenant}

	// Create parent project.
	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/projects", map[string]interface{}{
		"name": "engineering",
		"spec": map[string]interface{}{"Data": map[string]interface{}{"title": "Engineering"}},
	}, headers)
	readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Create child project with parent.
	parent := "engineering"
	resp = doRequest(t, http.MethodPost, baseURL+"/api/v1/projects", map[string]interface{}{
		"name":   "backend",
		"parent": &parent,
		"spec":   map[string]interface{}{"Data": map[string]interface{}{"title": "Backend Team"}},
	}, headers)
	body := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body: %s", string(body))
}

func TestDeleteProjectWithChildrenBlocked(t *testing.T) {
	baseURL, _, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()

	headers := map[string]string{"X-Org-ID": defaultTenant}

	doRequest(t, http.MethodPost, baseURL+"/api/v1/projects", map[string]interface{}{
		"name": "parent-proj",
		"spec": map[string]interface{}{"Data": map[string]interface{}{"title": "Parent"}},
	}, headers)

	parent := "parent-proj"
	doRequest(t, http.MethodPost, baseURL+"/api/v1/projects", map[string]interface{}{
		"name":   "child-proj",
		"parent": &parent,
		"spec":   map[string]interface{}{"Data": map[string]interface{}{"title": "Child"}},
	}, headers)

	// Deleting parent should be blocked.
	resp := doRequest(t, http.MethodDelete, baseURL+"/api/v1/projects/parent-proj", nil, headers)
	readBody(t, resp)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

// --- Tenant isolation ---

func TestTenantIsolation(t *testing.T) {
	baseURL, _, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()

	headersA := map[string]string{"X-Org-ID": defaultTenant}

	doRequest(t, http.MethodPost, baseURL+"/api/v1/host-types", map[string]interface{}{
		"name": "isolated-type",
		"spec": map[string]interface{}{"Data": map[string]interface{}{"title": "Isolated"}},
	}, headersA)

	// Different tenant should get 403 (guest tenancy only allows default tenant).
	headersB := map[string]string{"X-Org-ID": "99999999-9999-9999-9999-999999999999"}
	resp := doRequest(t, http.MethodGet, baseURL+"/api/v1/host-types/isolated-type", nil, headersB)
	readBody(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// --- Cross-resource validation ---

func TestSubnetWithoutVirtualNetworkRejected(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}

	// Create subnet referencing a nonexistent virtual network.
	payload := map[string]interface{}{
		"name": "orphan-subnet",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"virtual_network": "does-not-exist", "ipv4_cidr": "10.0.0.0/24",
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/subnets", payload, headers)
	body := readBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", string(body))
	assert.Contains(t, string(body), "not found")
}

func TestSecurityGroupWithoutVirtualNetworkRejected(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}

	payload := map[string]interface{}{
		"name": "orphan-sg",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"virtual_network": "nonexistent",
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/security-groups", payload, headers)
	body := readBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", string(body))
}

func TestPublicIPWithoutPoolRejected(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}

	payload := map[string]interface{}{
		"name": "orphan-ip",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"pool": "nonexistent-pool",
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/public-ips", payload, headers)
	body := readBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", string(body))
}

func TestClusterWithInvalidHostTypeRejected(t *testing.T) {
	baseURL, exec, cleanup := testutil.SetupOSACServer(t)
	defer cleanup()
	registerStubHandlers(exec)

	headers := map[string]string{"X-Org-ID": defaultTenant}

	payload := map[string]interface{}{
		"name": "bad-cluster",
		"spec": map[string]interface{}{"Data": map[string]interface{}{
			"release_image": "quay.io/openshift:4.16",
			"node_sets": []interface{}{
				map[string]interface{}{"name": "workers", "host_type": "nonexistent", "replicas": 3},
			},
		}},
	}

	resp := doRequest(t, http.MethodPost, baseURL+"/api/v1/clusters", payload, headers)
	body := readBody(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", string(body))
	assert.Contains(t, string(body), "not found")
}
