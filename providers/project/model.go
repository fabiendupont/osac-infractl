// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package project

import "github.com/fabiendupont/infractl/resource"

type ProjectSpec struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProjectStatus struct {
	Phase   string `json:"phase"` // Pending, Active, Failed, Deleting
	Message string `json:"message,omitempty"`
}

type Project struct {
	resource.Resource
	Spec   resource.JSONField[ProjectSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[ProjectStatus] `gorm:"type:jsonb" json:"status"`
}

func (Project) TableName() string { return "projects" }

func (p *Project) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(p.Spec.Data)
}
