// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/resource"
	"github.com/fabiendupont/infractl/workflow"
	"github.com/osac-project/osac-infractl/providers/blueprint"
)

func TestBlueprintExecution(t *testing.T) {
	table := workflow.NewDispatchTable()

	// Register handlers for two "catalog items" (using resource type as key).
	table.Register(workflow.Handler{
		ResourceType: "network-setup", Event: "create",
		Phase: workflow.PhaseMain, Ref: "create-network",
	})
	table.Register(workflow.Handler{
		ResourceType: "vm-provision", Event: "create",
		Phase: workflow.PhaseMain, Ref: "create-vm",
	})

	exec := workflow.NewLocalExecutor()
	exec.Register("create-network", func(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"network_id": "net-123",
			"subnet_id":  "sub-456",
		}, nil
	})
	exec.Register("create-vm", func(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"vm_id":     "vm-789",
			"subnet_id": input["subnet_id"],
		}, nil
	})

	logger := zerolog.Nop()
	dispatcher := workflow.NewDispatcher(table, exec, logger)

	bpExec := NewBlueprintExecutor(dispatcher, logger)

	bp := &blueprint.Blueprint{
		Resource: resource.Resource{
			OrgID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Name:  "test-blueprint",
		},
		Spec: resource.JSONField[blueprint.BlueprintSpec]{Data: blueprint.BlueprintSpec{
			Title: "Test Blueprint",
			Nodes: []blueprint.Node{
				{
					Name:        "network",
					CatalogItem: "network-setup",
				},
				{
					Name:        "vm",
					CatalogItem: "vm-provision",
					DependsOn:   []string{"network"},
					OutputWiring: map[string]string{
						"subnet_id": "network.subnet_id",
					},
				},
			},
		}},
	}

	results, err := bpExec.Execute(context.Background(), bp)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify network executed first.
	if results[0].Name != "network" {
		t.Errorf("results[0].Name = %q, want %q", results[0].Name, "network")
	}
	if results[0].Status != "Completed" {
		t.Errorf("results[0].Status = %q", results[0].Status)
	}
	if results[0].Outputs["network_id"] != "net-123" {
		t.Errorf("network output missing network_id")
	}

	// Verify VM received wired subnet_id.
	if results[1].Name != "vm" {
		t.Errorf("results[1].Name = %q, want %q", results[1].Name, "vm")
	}
	if results[1].Outputs["subnet_id"] != "sub-456" {
		t.Errorf("VM should have received subnet_id=sub-456 from network, got %v", results[1].Outputs["subnet_id"])
	}
}

func TestBlueprintCycleRejected(t *testing.T) {
	table := workflow.NewDispatchTable()
	exec := workflow.NewLocalExecutor()
	logger := zerolog.Nop()
	dispatcher := workflow.NewDispatcher(table, exec, logger)
	bpExec := NewBlueprintExecutor(dispatcher, logger)

	bp := &blueprint.Blueprint{
		Spec: resource.JSONField[blueprint.BlueprintSpec]{Data: blueprint.BlueprintSpec{
			Nodes: []blueprint.Node{
				{Name: "a", CatalogItem: "x", DependsOn: []string{"b"}},
				{Name: "b", CatalogItem: "y", DependsOn: []string{"a"}},
			},
		}},
	}

	_, err := bpExec.Execute(context.Background(), bp)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}
