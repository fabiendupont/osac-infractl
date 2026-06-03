// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package hosttype

import "github.com/fabiendupont/infractl/resource"

type HostTypeSpec struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type HostTypeStatus struct {
	Phase string `json:"phase"`
}

type HostType struct {
	resource.Resource
	Spec   resource.JSONField[HostTypeSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[HostTypeStatus] `gorm:"type:jsonb" json:"status"`
}

func (HostType) TableName() string { return "host_types" }

func (h *HostType) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(h.Spec.Data)
}
