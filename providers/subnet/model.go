// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package subnet

import "github.com/fabiendupont/infractl/resource"

type SubnetSpec struct {
	VirtualNetwork string `json:"virtual_network"`
	IPv4CIDR       string `json:"ipv4_cidr,omitempty"`
	IPv6CIDR       string `json:"ipv6_cidr,omitempty"`
}

type SubnetStatus struct {
	Phase   string `json:"phase"` // Pending, Ready, Failed, Deleting
	Message string `json:"message,omitempty"`
	Hub     string `json:"hub,omitempty"`
}

type Subnet struct {
	resource.Resource
	Spec   resource.JSONField[SubnetSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[SubnetStatus] `gorm:"type:jsonb" json:"status"`
}

func (Subnet) TableName() string { return "subnets" }

func (s *Subnet) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(s.Spec.Data)
}
