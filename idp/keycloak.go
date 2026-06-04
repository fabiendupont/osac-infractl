// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

// Package idp provides IdentityProvider implementations for OSAC.
package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fabiendupont/infractl/auth"
)

// KeycloakConfig holds the Keycloak Admin API connection parameters.
type KeycloakConfig struct {
	BaseURL      string
	AdminRealm   string // realm for admin auth (usually "master")
	ClientID     string
	ClientSecret string
	Timeout      time.Duration
}

// KeycloakProvider implements auth.IdentityProvider by provisioning
// Keycloak realms and break-glass admin accounts.
type KeycloakProvider struct {
	config     KeycloakConfig
	httpClient *http.Client

	mu         sync.Mutex
	token      string
	tokenExpiry time.Time
}

// NewKeycloakProvider creates a Keycloak identity provider.
func NewKeycloakProvider(cfg KeycloakConfig) *KeycloakProvider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if cfg.AdminRealm == "" {
		cfg.AdminRealm = "master"
	}
	return &KeycloakProvider{
		config:     cfg,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// ProvisionOrganization creates a Keycloak realm for the organization
// and a break-glass admin user with a temporary password.
func (p *KeycloakProvider) ProvisionOrganization(ctx context.Context, orgName, displayName string) (*auth.OrgProvisionResult, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("obtaining admin token: %w", err)
	}

	// Create realm.
	realm := map[string]interface{}{
		"realm":       orgName,
		"displayName": displayName,
		"enabled":     true,
	}
	if err := p.adminPost(ctx, token, "/admin/realms", realm); err != nil {
		return nil, fmt.Errorf("creating realm %q: %w", orgName, err)
	}

	// Create break-glass admin user.
	adminPassword := uuid.New().String()
	user := map[string]interface{}{
		"username":  "admin",
		"enabled":   true,
		"firstName": "Break Glass",
		"lastName":  "Admin",
		"credentials": []map[string]interface{}{
			{
				"type":      "password",
				"value":     adminPassword,
				"temporary": true,
			},
		},
	}
	if err := p.adminPost(ctx, token, fmt.Sprintf("/admin/realms/%s/users", orgName), user); err != nil {
		return nil, fmt.Errorf("creating admin user in realm %q: %w", orgName, err)
	}

	// Get the admin user ID.
	adminUserID, err := p.getUserID(ctx, token, orgName, "admin")
	if err != nil {
		adminUserID = "unknown"
	}

	return &auth.OrgProvisionResult{
		ExternalID:  orgName,
		AdminUserID: adminUserID,
		AdminSecret: adminPassword,
	}, nil
}

// DeprovisionOrganization deletes the Keycloak realm.
func (p *KeycloakProvider) DeprovisionOrganization(ctx context.Context, orgName string) error {
	token, err := p.getToken(ctx)
	if err != nil {
		return fmt.Errorf("obtaining admin token: %w", err)
	}

	url := fmt.Sprintf("%s/admin/realms/%s", p.config.BaseURL, orgName)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deleting realm %q: %w", orgName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleting realm %q: HTTP %d: %s", orgName, resp.StatusCode, string(body))
	}
	return nil
}

func (p *KeycloakProvider) getToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && time.Now().Add(30*time.Second).Before(p.tokenExpiry) {
		return p.token, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
	}

	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", p.config.BaseURL, p.config.AdminRealm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	p.token = tokenResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return p.token, nil
}

func (p *KeycloakProvider) adminPost(ctx context.Context, token, path string, body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := p.config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (p *KeycloakProvider) getUserID(ctx context.Context, token, realm, username string) (string, error) {
	url := fmt.Sprintf("%s/admin/realms/%s/users?username=%s&exact=true", p.config.BaseURL, realm, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var users []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return "", err
	}
	if len(users) == 0 {
		return "", fmt.Errorf("user %q not found in realm %q", username, realm)
	}
	return users[0].ID, nil
}

var _ auth.IdentityProvider = (*KeycloakProvider)(nil)
