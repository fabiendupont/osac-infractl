// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"testing"
)

func TestTopoSortLinear(t *testing.T) {
	nodes := []string{"c", "b", "a"}
	deps := map[string][]string{
		"c": {"b"},
		"b": {"a"},
		"a": {},
	}

	result, err := TopoSort(nodes, deps)
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}

	// a must come before b, b before c.
	idx := make(map[string]int)
	for i, n := range result {
		idx[n] = i
	}
	if idx["a"] > idx["b"] || idx["b"] > idx["c"] {
		t.Errorf("wrong order: %v", result)
	}
}

func TestTopoSortParallel(t *testing.T) {
	nodes := []string{"a", "b", "c"}
	deps := map[string][]string{
		"a": {},
		"b": {},
		"c": {"a", "b"},
	}

	result, err := TopoSort(nodes, deps)
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}

	idx := make(map[string]int)
	for i, n := range result {
		idx[n] = i
	}
	if idx["a"] > idx["c"] || idx["b"] > idx["c"] {
		t.Errorf("c should come after a and b: %v", result)
	}
}

func TestTopoSortCycleDetected(t *testing.T) {
	nodes := []string{"a", "b"}
	deps := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}

	_, err := TopoSort(nodes, deps)
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestTopoSortSingleNode(t *testing.T) {
	result, err := TopoSort([]string{"only"}, map[string][]string{"only": {}})
	if err != nil {
		t.Fatalf("TopoSort: %v", err)
	}
	if len(result) != 1 || result[0] != "only" {
		t.Errorf("result = %v", result)
	}
}
