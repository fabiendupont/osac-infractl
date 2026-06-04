// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package hub

import "github.com/fabiendupont/infractl/workflow"

// AddonManifest represents the meta/addon.yaml in an Ansible collection.
type AddonManifest struct {
	Name          string         `yaml:"name" json:"name"`
	DisplayName   string         `yaml:"display_name" json:"display_name"`
	Version       string         `yaml:"version" json:"version"`
	ResourceTypes []ResourceType `yaml:"resource_types" json:"resource_types"`
}

// ResourceType declares a resource type provided by the collection.
type ResourceType struct {
	Name  string `yaml:"name" json:"name"`
	Scope string `yaml:"scope" json:"scope"` // tenant, provider
}

// RoleMetadata represents the meta/osac.yaml in an Ansible role.
type RoleMetadata struct {
	ResourceType  string      `yaml:"resource_type" json:"resource_type"`
	Event         string      `yaml:"event" json:"event"`
	Phase         string      `yaml:"phase" json:"phase"`
	Priority      int         `yaml:"priority" json:"priority"`
	FailurePolicy string      `yaml:"failure_policy" json:"failure_policy"`
	Parameters    []Parameter `yaml:"parameters" json:"parameters,omitempty"`
	Outputs       []Output    `yaml:"outputs" json:"outputs,omitempty"`
}

// Parameter declares an input parameter for a role.
type Parameter struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Required bool   `yaml:"required" json:"required"`
	Default  string `yaml:"default" json:"default,omitempty"`
}

// Output declares an output from a role.
type Output struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
}

// ToHandler converts role metadata to an infractl workflow.Handler.
func (r *RoleMetadata) ToHandler(collection, roleName string) workflow.Handler {
	phase := workflow.PhaseMain
	switch r.Phase {
	case "pre":
		phase = workflow.PhasePre
	case "post":
		phase = workflow.PhasePost
	}

	priority := r.Priority
	if priority == 0 {
		priority = 100
	}

	return workflow.Handler{
		ResourceType: r.ResourceType,
		Event:        mapEvent(r.Event),
		Phase:        phase,
		Priority:     priority,
		Ref:          collection + "-" + roleName,
		Metadata: map[string]string{
			"collection":      collection,
			"resource_action": roleName,
			"failure_policy":  r.FailurePolicy,
		},
	}
}

func mapEvent(event string) string {
	switch event {
	case "Create", "create":
		return "create"
	case "Delete", "delete":
		return "delete"
	case "Update", "update":
		return "update"
	case "Signal", "signal":
		return "signal"
	default:
		return event
	}
}
