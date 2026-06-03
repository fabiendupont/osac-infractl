// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/fabiendupont/infractl/auth"
	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
)

type ProjectProvider struct {
	db     *gorm.DB
	store  resource.Store[Project]
	logger zerolog.Logger
}

func New() *ProjectProvider { return &ProjectProvider{} }

func (p *ProjectProvider) Name() string           { return "project" }
func (p *ProjectProvider) Version() string        { return "0.1.0" }
func (p *ProjectProvider) Features() []string     { return []string{"project"} }
func (p *ProjectProvider) Dependencies() []string { return []string{"tenant"} }

func (p *ProjectProvider) Init(ctx provider.Context) error {
	p.db = ctx.DB
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	p.store = resource.NewGenericStore[Project](ctx.DB)
	if err := ctx.DB.AutoMigrate(&Project{}); err != nil {
		return err
	}
	p.logger.Info().Msg("project provider initialized")
	return nil
}

func (p *ProjectProvider) Shutdown(_ context.Context) error { return nil }

func (p *ProjectProvider) RegisterRoutes(r chi.Router) {
	r.Route("/projects", func(r chi.Router) {
		r.Get("/", p.list)
		r.Post("/", p.create)
		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", p.get)
			r.Put("/", p.update)
			r.Delete("/", p.delete)
			r.Get("/children", p.listChildren)
		})
	})
}

func (p *ProjectProvider) list(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	list, err := p.store.List(r.Context(), orgID, resource.ListOptions{
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

func (p *ProjectProvider) get(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proj, err := p.store.Get(r.Context(), orgID, chi.URLParam(r, "name"))
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, proj)
}

func (p *ProjectProvider) create(w http.ResponseWriter, r *http.Request) {
	var proj Project
	if err := json.NewDecoder(r.Body).Decode(&proj); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proj.OrgID = orgID
	proj.Status = resource.JSONField[ProjectStatus]{
		Data: ProjectStatus{Phase: "Pending"},
	}

	if err := p.store.Create(r.Context(), &proj); err != nil {
		switch {
		case errors.Is(err, resource.ErrAlreadyExists):
			http.Error(w, "project already exists", http.StatusConflict)
		case errors.Is(err, resource.ErrParentNotFound):
			http.Error(w, "parent project not found", http.StatusBadRequest)
		case errors.Is(err, resource.ErrCircularParent):
			http.Error(w, "circular parent reference", http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusCreated, proj)
}

func (p *ProjectProvider) update(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	existing, err := p.store.Get(r.Context(), orgID, chi.URLParam(r, "name"))
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(existing); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := p.store.Update(r.Context(), existing); err != nil {
		switch {
		case errors.Is(err, resource.ErrConflict):
			http.Error(w, "resource version conflict", http.StatusConflict)
		case errors.Is(err, resource.ErrParentNotFound):
			http.Error(w, "parent project not found", http.StatusBadRequest)
		case errors.Is(err, resource.ErrCircularParent):
			http.Error(w, "circular parent reference", http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (p *ProjectProvider) delete(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := chi.URLParam(r, "name")

	if err := p.store.Delete(r.Context(), orgID, name); err != nil {
		switch {
		case errors.Is(err, resource.ErrNotFound):
			http.Error(w, "project not found", http.StatusNotFound)
		case errors.Is(err, resource.ErrHasChildren):
			http.Error(w, "project has child projects", http.StatusConflict)
		case errors.Is(err, resource.ErrFinalizersPending):
			w.WriteHeader(http.StatusAccepted)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (p *ProjectProvider) listChildren(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.OrgIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	name := chi.URLParam(r, "name")

	children, err := resource.ListChildren(r.Context(), p.db, orgID, name, &Project{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"parent":   name,
		"children": children,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

var _ provider.Provider = (*ProjectProvider)(nil)
var _ provider.APIProvider = (*ProjectProvider)(nil)
