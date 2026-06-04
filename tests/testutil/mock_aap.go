// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// MockJob tracks a simulated AAP job.
type MockJob struct {
	ID         int
	Status     string
	Template   string
	ExtraVars  map[string]interface{}
	PollCount  int
	ReadyAfter int
}

// MockAAP simulates the AAP Controller v2 API for testing the full
// dispatch → executor → poll → status update loop.
type MockAAP struct {
	Server *httptest.Server

	mu      sync.Mutex
	nextID  atomic.Int32
	jobs    map[int]*MockJob
	readyAfter int
}

// NewMockAAP creates a mock AAP server. Jobs transition to "successful"
// after readyAfter poll requests.
func NewMockAAP(t *testing.T, readyAfter int) *MockAAP {
	t.Helper()

	mock := &MockAAP{
		jobs:       make(map[int]*MockJob),
		readyAfter: readyAfter,
	}
	mock.nextID.Store(100)

	mux := http.NewServeMux()

	// POST /api/v2/job_templates/{name}/launch/
	mux.HandleFunc("/api/v2/job_templates/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/launch/") {
			http.NotFound(w, r)
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		templateName := parts[3]

		var body struct {
			ExtraVars map[string]interface{} `json:"extra_vars"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		id := int(mock.nextID.Add(1))
		mock.mu.Lock()
		mock.jobs[id] = &MockJob{
			ID:         id,
			Status:     "pending",
			Template:   templateName,
			ExtraVars:  body.ExtraVars,
			ReadyAfter: mock.readyAfter,
		}
		mock.mu.Unlock()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id})
	})

	// GET /api/v2/jobs/{id}/
	mux.HandleFunc("/api/v2/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel/") {
			// Cancel
			w.WriteHeader(http.StatusAccepted)
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 4 {
			http.NotFound(w, r)
			return
		}

		var jobID int
		fmt.Sscanf(parts[3], "%d", &jobID)

		mock.mu.Lock()
		job, ok := mock.jobs[jobID]
		if ok {
			job.PollCount++
			if job.PollCount >= job.ReadyAfter {
				job.Status = "successful"
			} else {
				job.Status = "running"
			}
		}
		mock.mu.Unlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     job.ID,
			"status": job.Status,
		})
	})

	// GET /api/v2/ping/
	mux.HandleFunc("/api/v2/ping/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ha":false,"version":"mock"}`))
	})

	mock.Server = httptest.NewServer(mux)
	return mock
}

// Close shuts down the mock server.
func (m *MockAAP) Close() {
	m.Server.Close()
}

// URL returns the base URL of the mock server.
func (m *MockAAP) URL() string {
	return m.Server.URL
}

// Jobs returns all jobs that were submitted.
func (m *MockAAP) Jobs() map[int]*MockJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[int]*MockJob, len(m.jobs))
	for k, v := range m.jobs {
		result[k] = v
	}
	return result
}

// JobCount returns the number of jobs submitted.
func (m *MockAAP) JobCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.jobs)
}
