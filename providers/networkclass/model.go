// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package networkclass

import "github.com/fabiendupont/infractl/resource"

type NetworkClassSpec struct {
	Title                  string `json:"title,omitempty"`
	Description            string `json:"description,omitempty"`
	ImplementationStrategy string `json:"implementation_strategy,omitempty"`
	SupportsIPv4           bool   `json:"supports_ipv4"`
	SupportsIPv6           bool   `json:"supports_ipv6"`
	SupportsDualStack      bool   `json:"supports_dual_stack"`
}

type NetworkClassStatus struct {
	Phase string `json:"phase"`
}

type NetworkClass struct {
	resource.Resource
	Spec   resource.JSONField[NetworkClassSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[NetworkClassStatus] `gorm:"type:jsonb" json:"status"`
}

func (NetworkClass) TableName() string { return "network_classes" }

func (n *NetworkClass) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(n.Spec.Data)
}
