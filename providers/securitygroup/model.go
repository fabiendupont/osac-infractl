// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package securitygroup

import "github.com/fabiendupont/infractl/resource"

type Rule struct {
	Protocol string `json:"protocol"`
	PortFrom int    `json:"port_from"`
	PortTo   int    `json:"port_to"`
	IPv4CIDR string `json:"ipv4_cidr,omitempty"`
	IPv6CIDR string `json:"ipv6_cidr,omitempty"`
}

type SecurityGroupSpec struct {
	VirtualNetwork string `json:"virtual_network"`
	Ingress        []Rule `json:"ingress,omitempty"`
	Egress         []Rule `json:"egress,omitempty"`
}

type SecurityGroupStatus struct {
	Phase   string `json:"phase"`
	Message string `json:"message,omitempty"`
}

type SecurityGroup struct {
	resource.Resource
	Spec   resource.JSONField[SecurityGroupSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[SecurityGroupStatus] `gorm:"type:jsonb" json:"status"`
}

func (SecurityGroup) TableName() string { return "security_groups" }

func (s *SecurityGroup) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(s.Spec.Data)
}
