// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package computeinstance

import "github.com/fabiendupont/infractl/resource"

type NetworkAttachment struct {
	Subnet         string   `json:"subnet"`
	SecurityGroups []string `json:"security_groups,omitempty"`
}

type Disk struct {
	SizeGB int    `json:"size_gb"`
	Type   string `json:"type,omitempty"`
}

type ComputeInstanceSpec struct {
	Template           string              `json:"template,omitempty"`
	TemplateParameters string              `json:"template_parameters,omitempty"`
	Image              string              `json:"image,omitempty"`
	Cores              int                 `json:"cores"`
	MemoryGiB          int                 `json:"memory_gib"`
	BootDisk           Disk                `json:"boot_disk"`
	AdditionalDisks    []Disk              `json:"additional_disks,omitempty"`
	NetworkAttachments []NetworkAttachment `json:"network_attachments,omitempty"`
	SSHKeyRefs         []string            `json:"ssh_key_refs,omitempty"`
	Region             string              `json:"region,omitempty"`
}

type ComputeInstanceStatus struct {
	Phase      string `json:"phase"` // Starting, Running, Failed, Deleting, Stopping, Stopped, Paused
	Message    string `json:"message,omitempty"`
	InternalIP string `json:"internal_ip,omitempty"`
	PublicIP   string `json:"public_ip,omitempty"`
	Hub        string `json:"hub,omitempty"`
}

type ComputeInstance struct {
	resource.Resource
	Spec   resource.JSONField[ComputeInstanceSpec]   `gorm:"type:jsonb" json:"spec"`
	Status resource.JSONField[ComputeInstanceStatus] `gorm:"type:jsonb" json:"status"`
}

func (ComputeInstance) TableName() string { return "compute_instances" }

func (c *ComputeInstance) SpecBytes() ([]byte, error) {
	return resource.MarshalSpec(c.Spec.Data)
}
