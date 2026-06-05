// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package blueprint

import "github.com/fabiendupont/infractl/resource"

// Node represents a single resource in the blueprint DAG.
type Node struct {
	Name          string            `json:"name"`
	CatalogItem   string            `json:"catalog_item"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	DependsOn     []string          `json:"depends_on,omitempty"`
	OutputWiring  map[string]string `json:"output_wiring,omitempty"`
}

// BlueprintSpec defines the DAG of resources to provision.
type BlueprintSpec struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Nodes       []Node `json:"nodes"`
}

// NodeStatus tracks the provisioning status of a single node.
type NodeStatus struct {
	Name    string            `json:"name"`
	Phase   string            `json:"phase"` // Pending, Provisioning, Ready, Failed
	Outputs map[string]string `json:"outputs,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// BlueprintStatus tracks the overall blueprint execution.
type BlueprintStatus struct {
	Phase       string       `json:"phase"` // Pending, Provisioning, Ready, Failed, RollingBack
	Message     string       `json:"message,omitempty"`
	NodeStatus  []NodeStatus `json:"node_status,omitempty"`
}

// Blueprint defines a DAG of CatalogItems to provision as a unit.
// Nodes execute in dependency order; outputs wire between nodes
// via the output_wiring map (target_param → source_node.output_name).
type Blueprint struct {
	resource.Resource
	Spec   resource.JSONField[BlueprintSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[BlueprintStatus] `gorm:"type:jsonb" json:"status"`
}

func (Blueprint) TableName() string { return "blueprints" }

func (b *Blueprint) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(b.Spec.Data)
}
