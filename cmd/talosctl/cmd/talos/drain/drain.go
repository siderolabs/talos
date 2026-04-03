// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package drain implements Kubernetes node drain progress reporting.
package drain

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/siderolabs/talos/pkg/reporter"
)

// ProgressWriter writes drain progress updates to a reporter.
//
// It is NOT thread-safe on its own. Callers must ensure that UpdateNode and
// PrintProgress are called from a single goroutine (e.g. via a channel-based
// aggregator, matching the action tracker pattern).
type ProgressWriter struct {
	// nodeStates keeps track of the current drain state per node.
	nodeStates map[string]nodeState
}

// UpdateNode updates the drain progress for a given node.
func (w *ProgressWriter) UpdateNode(node, message string, status reporter.Status) {
	if w.nodeStates == nil {
		w.nodeStates = make(map[string]nodeState)
	}

	w.nodeStates[node] = nodeState{
		message: message,
		status:  status,
	}
}

// PrintProgress prints the current drain progress for all nodes to the reporter.
func (w *ProgressWriter) PrintProgress(rep *reporter.Reporter) {
	nodes := slices.Collect(maps.Keys(w.nodeStates))
	sort.Strings(nodes)

	sb := strings.Builder{}

	for _, node := range nodes {
		state := w.nodeStates[node]
		fmt.Fprintf(&sb, "%s\n", state.message)
	}

	// Compute the combined status: error > running > succeeded.
	combined := reporter.StatusSucceeded

	for _, state := range w.nodeStates {
		if state.status == reporter.StatusError {
			combined = reporter.StatusError

			break
		}

		if state.status == reporter.StatusRunning {
			combined = reporter.StatusRunning
		}
	}

	rep.Report(reporter.Update{
		Message: sb.String(),
		Status:  combined,
	})
}

type nodeState struct {
	message string
	status  reporter.Status
}
