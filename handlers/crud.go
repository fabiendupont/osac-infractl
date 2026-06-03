// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

// Package handlers provides a generic CRUD handler factory that eliminates
// per-provider handler boilerplate. Each provider only needs to define its
// model and register routes — the actual HTTP handling is generated.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/fabiendupont/infractl/auth"
	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
)

// ResourceFactory creates a new zero-value instance of a resource type
// with default status set.
type ResourceFactory[R any] func() *R

// CRUDHandler provides standard CRUD HTTP handlers for any resource type.
// If Hooks is set, lifecycle events are fired after successful mutations.
type CRUDHandler[R any] struct {
	Store        resource.Store[R]
	NewItem      ResourceFactory[R]
	Hooks        provider.HookFirer
	ResourceType string
}

// NewCRUDHandler creates a handler set for the given store.
func NewCRUDHandler[R any](store resource.Store[R], factory ResourceFactory[R]) *CRUDHandler[R] {
	return &CRUDHandler[R]{Store: store, NewItem: factory}
}

// WithHooks sets the hook firer and resource type for lifecycle events.
func (h *CRUDHandler[R]) WithHooks(hooks provider.HookFirer, resourceType string) *CRUDHandler[R] {
	h.Hooks = hooks
	h.ResourceType = resourceType
	return h
}

func (h *CRUDHandler[R]) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	list, err := h.Store.List(r.Context(), orgID, resource.ListOptions{
		Limit:    limit,
		Continue: r.URL.Query().Get("continue"),
		Filter:   r.URL.Query().Get("filter"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *CRUDHandler[R]) Get(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	item, err := h.Store.Get(r.Context(), orgID, chi.URLParam(r, "name"))
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *CRUDHandler[R]) Create(w http.ResponseWriter, r *http.Request) {
	item := h.NewItem()
	if err := json.NewDecoder(r.Body).Decode(item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if accessor, ok := any(item).(resource.ResourceAccessor); ok {
		accessor.(*resource.Resource).OrgID = orgID
	}

	if h.Hooks != nil && h.ResourceType != "" {
		if err := h.Hooks.FireSync(r.Context(), h.ResourceType, "pre_create", item); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	if err := h.Store.Create(r.Context(), item); err != nil {
		if errors.Is(err, resource.ErrAlreadyExists) {
			http.Error(w, "already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.Hooks != nil && h.ResourceType != "" {
		h.Hooks.FireAsync(r.Context(), h.ResourceType, "post_create", item)
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *CRUDHandler[R]) Update(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	existing, err := h.Store.Get(r.Context(), orgID, chi.URLParam(r, "name"))
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(existing); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if h.Hooks != nil && h.ResourceType != "" {
		if err := h.Hooks.FireSync(r.Context(), h.ResourceType, "pre_update", existing); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	if err := h.Store.Update(r.Context(), existing); err != nil {
		if errors.Is(err, resource.ErrConflict) {
			http.Error(w, "conflict", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.Hooks != nil && h.ResourceType != "" {
		h.Hooks.FireAsync(r.Context(), h.ResourceType, "post_update", existing)
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *CRUDHandler[R]) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := chi.URLParam(r, "name")

	if h.Hooks != nil && h.ResourceType != "" {
		if err := h.Hooks.FireSync(r.Context(), h.ResourceType, "pre_delete", map[string]interface{}{
			"org_id": orgID.String(), "name": name,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	if err := h.Store.Delete(r.Context(), orgID, name); err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, resource.ErrFinalizersPending) {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if errors.Is(err, resource.ErrHasChildren) {
			http.Error(w, "has children", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.Hooks != nil && h.ResourceType != "" {
		h.Hooks.FireAsync(r.Context(), h.ResourceType, "post_delete", map[string]interface{}{
			"org_id": orgID.String(), "name": name,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegisterRoutes mounts standard CRUD routes on the given path.
func (h *CRUDHandler[R]) RegisterRoutes(r chi.Router, path string) {
	r.Route(path, func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Put("/", h.Update)
			r.Delete("/", h.Delete)
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
