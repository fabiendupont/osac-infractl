// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package publicip

import "github.com/fabiendupont/infractl/resource"

type PublicIPSpec struct {
	Pool string `json:"pool"`
}

type PublicIPStatus struct {
	Phase    string `json:"phase"` // Pending, Allocated, Failed, Deleting
	Address  string `json:"address,omitempty"`
	Hub      string `json:"hub,omitempty"`
	Attached bool   `json:"attached"`
}

type PublicIP struct {
	resource.Resource
	Spec   resource.JSONField[PublicIPSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[PublicIPStatus] `gorm:"type:jsonb" json:"status"`
}

func (PublicIP) TableName() string { return "public_ips" }

func (p *PublicIP) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(p.Spec.Data)
}
