// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package computeinstance

import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/fabiendupont/infractl/provider"
	"github.com/fabiendupont/infractl/resource"
	"github.com/fabiendupont/infractl/workflow"
	"github.com/osac-project/osac-infractl/handlers"
)

const (
	ResourceType = "ComputeInstance"
	Collection   = "osac.compute_kubevirt"
)

type ComputeInstanceProvider struct {
	crud   *handlers.CRUDHandler[ComputeInstance]
	logger zerolog.Logger
}

func New() *ComputeInstanceProvider { return &ComputeInstanceProvider{} }

func (p *ComputeInstanceProvider) Name() string           { return "computeinstance" }
func (p *ComputeInstanceProvider) Version() string        { return "0.1.0" }
func (p *ComputeInstanceProvider) Features() []string     { return []string{"computeinstance"} }
func (p *ComputeInstanceProvider) Dependencies() []string { return []string{"subnet", "securitygroup"} }

func (p *ComputeInstanceProvider) Init(ctx provider.Context) error {
	p.logger = ctx.Logger.With().Str("provider", p.Name()).Logger()
	store := resource.NewGenericStore[ComputeInstance](ctx.DB)
	p.crud = handlers.NewCRUDHandler[ComputeInstance](store, func() *ComputeInstance {
		return &ComputeInstance{Status: resource.JSONField[ComputeInstanceStatus]{Data: ComputeInstanceStatus{Phase: "Starting"}}}
	})
	if ctx.Hooks != nil {
		p.crud.WithHooks(ctx.Hooks, ResourceType)
	}
	if err := ctx.DB.AutoMigrate(&ComputeInstance{}); err != nil {
		return err
	}

	if ctx.Registry != nil {
		db := ctx.DB
		ctx.Registry.RegisterHook(provider.SyncHook{
			Feature: ResourceType,
			Event:   "pre_create",
			Handler: validateNetworkAttachments(db),
		})
	}

	p.logger.Info().Msg("compute instance provider initialized")
	return nil
}

func validateNetworkAttachments(db *gorm.DB) func(context.Context, interface{}) error {
	return func(ctx context.Context, payload interface{}) error {
		ci, ok := payload.(*ComputeInstance)
		if !ok {
			return nil
		}
		for i, att := range ci.Spec.Data.NetworkAttachments {
			if att.Subnet == "" {
				return fmt.Errorf("network_attachments[%d].subnet is required", i)
			}
			var count int64
			if err := db.WithContext(ctx).Table("subnets").
				Where("org_id = ? AND name = ? AND deleted_at IS NULL", ci.OrgID, att.Subnet).
				Count(&count).Error; err != nil {
				return fmt.Errorf("validating subnet: %w", err)
			}
			if count == 0 {
				return fmt.Errorf("Subnet %q not found (referenced by network_attachments[%d])", att.Subnet, i)
			}
			for j, sg := range att.SecurityGroups {
				if err := db.WithContext(ctx).Table("security_groups").
					Where("org_id = ? AND name = ? AND deleted_at IS NULL", ci.OrgID, sg).
					Count(&count).Error; err != nil {
					return fmt.Errorf("validating security group: %w", err)
				}
				if count == 0 {
					return fmt.Errorf("SecurityGroup %q not found (referenced by network_attachments[%d].security_groups[%d])", sg, i, j)
				}
			}
		}
		return nil
	}
}

func (p *ComputeInstanceProvider) Shutdown(_ context.Context) error { return nil }

func (p *ComputeInstanceProvider) RegisterRoutes(r chi.Router) {
	p.crud.RegisterRoutes(r, "/compute-instances")
}

func (p *ComputeInstanceProvider) RegisterActions(table *workflow.DispatchTable) {
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "create", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-create", Metadata: map[string]string{"collection": Collection, "resource_action": "instance"},
	})
	table.Register(workflow.Handler{
		ResourceType: ResourceType, Event: "delete", Phase: workflow.PhaseMain, Priority: 100,
		Ref: Collection + "-delete", Metadata: map[string]string{"collection": Collection, "resource_action": "instance"},
	})
}

var _ provider.Provider = (*ComputeInstanceProvider)(nil)
var _ provider.APIProvider = (*ComputeInstanceProvider)(nil)
var _ provider.WorkflowProvider = (*ComputeInstanceProvider)(nil)
