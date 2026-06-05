// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

// Package orchestrator executes Blueprint DAGs by dispatching each node
// in dependency order, wiring outputs between nodes.
package orchestrator

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/fabiendupont/infractl/workflow"
	"github.com/osac-project/osac-infractl/providers/blueprint"
)

// NodeResult holds the execution result for a single blueprint node.
type NodeResult struct {
	Name    string
	Status  string
	Outputs map[string]interface{}
	Error   string
}

// BlueprintExecutor orchestrates a Blueprint's DAG of nodes.
type BlueprintExecutor struct {
	dispatcher *workflow.Dispatcher
	logger     zerolog.Logger
}

// NewBlueprintExecutor creates a blueprint executor.
func NewBlueprintExecutor(dispatcher *workflow.Dispatcher, logger zerolog.Logger) *BlueprintExecutor {
	return &BlueprintExecutor{
		dispatcher: dispatcher,
		logger:     logger.With().Str("component", "blueprint-executor").Logger(),
	}
}

// Execute runs all nodes in the blueprint in dependency order.
// Returns per-node results and an overall error if any node fails.
func (e *BlueprintExecutor) Execute(ctx context.Context, bp *blueprint.Blueprint) ([]NodeResult, error) {
	spec := bp.Spec.Data

	// Build dependency map and node name list.
	nodeNames := make([]string, len(spec.Nodes))
	deps := make(map[string][]string)
	nodeMap := make(map[string]*blueprint.Node)
	for i := range spec.Nodes {
		n := &spec.Nodes[i]
		nodeNames[i] = n.Name
		deps[n.Name] = n.DependsOn
		nodeMap[n.Name] = n
	}

	// Topological sort.
	order, err := TopoSort(nodeNames, deps)
	if err != nil {
		return nil, fmt.Errorf("blueprint DAG invalid: %w", err)
	}

	// Execute nodes in order, collecting outputs.
	outputs := make(map[string]map[string]interface{})
	var results []NodeResult

	for _, name := range order {
		node := nodeMap[name]

		// Build input by merging node parameters with wired outputs.
		input := make(map[string]interface{})
		for k, v := range node.Parameters {
			input[k] = v
		}
		for targetParam, sourceRef := range node.OutputWiring {
			val, err := resolveOutput(sourceRef, outputs)
			if err != nil {
				e.logger.Warn().Err(err).Str("node", name).Str("param", targetParam).Msg("output wiring failed")
				continue
			}
			input[targetParam] = val
		}

		input["catalog_item"] = node.CatalogItem
		input["blueprint"] = bp.Name
		input["node"] = name

		e.logger.Info().Str("node", name).Str("catalog_item", node.CatalogItem).Msg("executing node")

		// Dispatch the node as a "create" event for its catalog item's resource type.
		// The CatalogItem's provisioning metadata determines the actual handler.
		run, err := e.dispatcher.Dispatch(ctx, node.CatalogItem, "create", input)

		result := NodeResult{Name: name}
		if err != nil {
			result.Status = "Failed"
			result.Error = err.Error()
			results = append(results, result)
			e.logger.Error().Err(err).Str("node", name).Msg("node failed")
			return results, fmt.Errorf("node %q failed: %w", name, err)
		}

		if run != nil {
			result.Status = "Completed"
			result.Outputs = run.Outputs
			outputs[name] = run.Outputs
		} else {
			result.Status = "Completed"
		}

		results = append(results, result)
		e.logger.Info().Str("node", name).Str("status", result.Status).Msg("node completed")
	}

	return results, nil
}

// resolveOutput parses "source_node.output_name" and looks up the value.
func resolveOutput(ref string, outputs map[string]map[string]interface{}) (interface{}, error) {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '.' {
			nodeName := ref[:i]
			outputName := ref[i+1:]
			nodeOutputs, ok := outputs[nodeName]
			if !ok {
				return nil, fmt.Errorf("node %q has no outputs", nodeName)
			}
			val, ok := nodeOutputs[outputName]
			if !ok {
				return nil, fmt.Errorf("node %q has no output %q", nodeName, outputName)
			}
			return val, nil
		}
	}
	return nil, fmt.Errorf("invalid output reference %q (expected node.output)", ref)
}
