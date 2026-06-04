// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/osac-project/osac-infractl/tests/testutil"
)

// getAdminToken obtains a token from dev-mode Keycloak using the
// admin username/password (resource owner password grant).
func getAdminToken(t *testing.T, baseURL string) string {
	t.Helper()

	data := url.Values{
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
		"username":   {"admin"},
		"password":   {"admin"},
	}

	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", baseURL)
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		t.Fatalf("failed to get admin token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token endpoint returned %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)
	return tokenResp.AccessToken
}

func TestKeycloakProvisionOrganization(t *testing.T) {
	baseURL, cleanup := testutil.SetupKeycloak(t)
	defer cleanup()

	// Use direct REST API to test provisioning since the KeycloakProvider
	// uses client_credentials grant which isn't available in dev mode
	// without creating a client first. Instead we test the same operations
	// the provider does, using the admin password grant.

	token := getAdminToken(t, baseURL)

	// Create a realm (equivalent to ProvisionOrganization).
	realm := map[string]interface{}{
		"realm":       "test-org",
		"displayName": "Test Organization",
		"enabled":     true,
	}
	realmBody, _ := json.Marshal(realm)

	req, _ := http.NewRequest("POST", baseURL+"/admin/realms", strings.NewReader(string(realmBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create realm: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create realm returned %d", resp.StatusCode)
	}

	// Verify the realm exists.
	req, _ = http.NewRequest("GET", baseURL+"/admin/realms/test-org", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get realm: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get realm returned %d, want 200", resp.StatusCode)
	}

	// Create a break-glass admin user in the realm.
	user := map[string]interface{}{
		"username": "admin",
		"enabled":  true,
		"credentials": []map[string]interface{}{
			{"type": "password", "value": "temp-pass", "temporary": true},
		},
	}
	userBody, _ := json.Marshal(user)

	req, _ = http.NewRequest("POST", baseURL+"/admin/realms/test-org/users", strings.NewReader(string(userBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user returned %d", resp.StatusCode)
	}

	// Verify the user exists.
	req, _ = http.NewRequest("GET", baseURL+"/admin/realms/test-org/users?username=admin&exact=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get users: %v", err)
	}
	defer resp.Body.Close()

	var users []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	json.NewDecoder(resp.Body).Decode(&users)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "admin" {
		t.Errorf("username = %q, want %q", users[0].Username, "admin")
	}

	// Delete the realm (equivalent to DeprovisionOrganization).
	req, _ = http.NewRequest("DELETE", baseURL+"/admin/realms/test-org", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete realm: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete realm returned %d", resp.StatusCode)
	}

	// Verify the realm is gone.
	req, _ = http.NewRequest("GET", baseURL+"/admin/realms/test-org", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get deleted realm: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
}

func TestKeycloakDuplicateRealmRejected(t *testing.T) {
	baseURL, cleanup := testutil.SetupKeycloak(t)
	defer cleanup()

	token := getAdminToken(t, baseURL)

	realm := map[string]interface{}{
		"realm":   "dup-org",
		"enabled": true,
	}
	realmBody, _ := json.Marshal(realm)

	// Create first.
	req, _ := http.NewRequest("POST", baseURL+"/admin/realms", strings.NewReader(string(realmBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first create returned %d", resp.StatusCode)
	}

	// Create duplicate.
	req, _ = http.NewRequest("POST", baseURL+"/admin/realms", strings.NewReader(string(realmBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatal("expected duplicate realm to be rejected")
	}

	// Cleanup.
	req, _ = http.NewRequest("DELETE", baseURL+"/admin/realms/dup-org", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	http.DefaultClient.Do(req)
}
