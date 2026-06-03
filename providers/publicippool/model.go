// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package publicippool

import "github.com/fabiendupont/infractl/resource"

type PublicIPPoolSpec struct {
	CIDRs                  []string `json:"cidrs"`
	IPFamily               string   `json:"ip_family"` // IPv4, IPv6
	ImplementationStrategy string   `json:"implementation_strategy,omitempty"`
}

type PublicIPPoolStatus struct {
	Phase     string `json:"phase"` // Pending, Ready, Failed, Deleting
	Hub       string `json:"hub,omitempty"`
	Total     int    `json:"total"`
	Allocated int    `json:"allocated"`
	Available int    `json:"available"`
}

type PublicIPPool struct {
	resource.Resource
	Spec   resource.JSONField[PublicIPPoolSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[PublicIPPoolStatus] `gorm:"type:jsonb" json:"status"`
}

func (PublicIPPool) TableName() string { return "public_ip_pools" }

func (p *PublicIPPool) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(p.Spec.Data)
}
