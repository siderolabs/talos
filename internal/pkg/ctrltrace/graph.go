// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package ctrltrace

import (
	"github.com/cosi-project/runtime/pkg/controller"
)

//nolint:gochecknoglobals
var edgeTypeNames = map[controller.DependencyEdgeType]string{
	controller.EdgeOutputExclusive:          "output-exclusive",
	controller.EdgeOutputShared:             "output-shared",
	controller.EdgeInputStrong:              "input-strong",
	controller.EdgeInputWeak:                "input-weak",
	controller.EdgeInputDestroyReady:        "input-destroy-ready",
	controller.EdgeInputQPrimary:            "input-q-primary",
	controller.EdgeInputQMapped:             "input-q-mapped",
	controller.EdgeInputQMappedDestroyReady: "input-q-mapped-destroy-ready",
}

// EmitGraph records the static controller/resource dependency graph into the trace.
//
// This is the same graph `talosctl inspect dependencies` renders; embedding it into the
// trace makes the trace file self-contained.
func EmitGraph(graph *controller.DependencyGraph) {
	t := Global()
	if t == nil || graph == nil {
		return
	}

	edges := make([]map[string]string, 0, len(graph.Edges))

	for _, edge := range graph.Edges {
		typ, ok := edgeTypeNames[edge.EdgeType]
		if !ok {
			typ = "unknown"
		}

		e := map[string]string{
			"ctrl": edge.ControllerName,
			"type": typ,
			"ns":   edge.ResourceNamespace,
			"rt":   edge.ResourceType,
		}

		if edge.ResourceID != "" {
			e["id"] = edge.ResourceID
		}

		edges = append(edges, e)
	}

	t.Emit(Event{
		K:     "graph",
		Op:    "dependencies",
		Extra: map[string]any{"edges": edges},
	})
}
