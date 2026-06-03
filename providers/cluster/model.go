// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package cluster

import "github.com/fabiendupont/infractl/resource"

type NodeSet struct {
	Name     string `json:"name"`
	HostType string `json:"host_type"`
	Replicas int    `json:"replicas"`
}

type ClusterSpec struct {
	Template           string   `json:"template,omitempty"`
	TemplateParameters string   `json:"template_parameters,omitempty"`
	NodeSets           []NodeSet `json:"node_sets,omitempty"`
	PullSecret         string   `json:"pull_secret,omitempty"`
	SSHPublicKey       string   `json:"ssh_public_key,omitempty"`
	ReleaseImage       string   `json:"release_image,omitempty"`
}

type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type ClusterStatus struct {
	Phase      string      `json:"phase"` // Progressing, Ready, Failed
	Conditions []Condition `json:"conditions,omitempty"`
	APIURL     string      `json:"api_url,omitempty"`
	ConsoleURL string      `json:"console_url,omitempty"`
	Hub        string      `json:"hub,omitempty"`
}

type Cluster struct {
	resource.Resource
	Spec   resource.JSONField[ClusterSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[ClusterStatus] `gorm:"type:jsonb" json:"status"`
}

func (Cluster) TableName() string { return "clusters" }

func (c *Cluster) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(c.Spec.Data)
}
