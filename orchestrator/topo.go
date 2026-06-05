// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import "fmt"

// TopoSort returns nodes in dependency order. Returns an error if
// the graph has a cycle.
func TopoSort(nodes []string, deps map[string][]string) ([]string, error) {
	state := make(map[string]int) // 0=unvisited, 1=visiting, 2=visited
	var result []string

	var visit func(n string) error
	visit = func(n string) error {
		if state[n] == 2 {
			return nil
		}
		if state[n] == 1 {
			return fmt.Errorf("cycle detected at %q", n)
		}
		state[n] = 1
		for _, dep := range deps[n] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[n] = 2
		result = append(result, n)
		return nil
	}

	for _, n := range nodes {
		if err := visit(n); err != nil {
			return nil, err
		}
	}

	return result, nil
}
