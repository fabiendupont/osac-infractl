// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package virtualnetwork

import "github.com/fabiendupont/infractl/resource"

type VirtualNetworkSpec struct {
	NetworkClass string `json:"network_class,omitempty"`
}

type VirtualNetworkStatus struct {
	Phase   string `json:"phase"` // Pending, Ready, Failed, Deleting
	Message string `json:"message,omitempty"`
	Hub     string `json:"hub,omitempty"`
}

type VirtualNetwork struct {
	resource.Resource
	Spec   resource.JSONField[VirtualNetworkSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[VirtualNetworkStatus] `gorm:"type:jsonb" json:"status"`
}

func (VirtualNetwork) TableName() string { return "virtual_networks" }

func (v *VirtualNetwork) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(v.Spec.Data)
}
