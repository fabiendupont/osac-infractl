// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

// Package hub discovers OSAC add-on collections from Automation Hub
// and populates the workflow dispatch table from their metadata.
package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientConfig holds the Automation Hub connection parameters.
type ClientConfig struct {
	BaseURL    string
	Token      string
	Namespaces []string // e.g., ["osac", "netris", "massopencloud"]
	Timeout    time.Duration
}

// Client talks to the Automation Hub v3 API.
type Client struct {
	baseURL    string
	token      string
	namespaces []string
	httpClient *http.Client
}

// NewClient creates an Automation Hub client.
func NewClient(cfg ClientConfig) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	namespaces := cfg.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{"osac"}
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		token:      cfg.Token,
		namespaces: namespaces,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Collection represents a collection from the Hub API.
type Collection struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Version   string `json:"highest_version"`
}

// CollectionListResponse is the paginated response from the Hub API.
type CollectionListResponse struct {
	Data  []Collection `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

// ListCollections returns all collections in the configured namespaces.
func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	var all []Collection

	for _, ns := range c.namespaces {
		url := fmt.Sprintf("%s/api/galaxy/v3/collections/?namespace=%s&limit=100", c.baseURL, ns)

		for url != "" {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			c.setHeaders(req)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("listing collections for namespace %s: %w", ns, err)
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("hub returned HTTP %d: %s", resp.StatusCode, string(body))
			}

			var listResp CollectionListResponse
			if err := json.Unmarshal(body, &listResp); err != nil {
				return nil, fmt.Errorf("decoding collection list: %w", err)
			}

			all = append(all, listResp.Data...)
			url = listResp.Links.Next
		}
	}

	return all, nil
}

// CollectionDetail holds the full metadata for a collection version.
type CollectionDetail struct {
	Namespace   string          `json:"namespace"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Metadata    json.RawMessage `json:"metadata"`
	ContentType string          `json:"content_type"`
}

// GetCollectionVersion fetches details for a specific collection version.
func (c *Client) GetCollectionVersion(ctx context.Context, namespace, name, version string) (*CollectionDetail, error) {
	url := fmt.Sprintf("%s/api/galaxy/v3/collections/%s/%s/versions/%s/", c.baseURL, namespace, name, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting collection %s.%s %s: %w", namespace, name, version, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hub returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var detail CollectionDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decoding collection detail: %w", err)
	}
	return &detail, nil
}

func (c *Client) setHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Token "+c.token)
	}
	req.Header.Set("Accept", "application/json")
}
