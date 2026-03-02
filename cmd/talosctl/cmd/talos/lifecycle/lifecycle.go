// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lifecycle implements image install progress reporting.
package lifecycle

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/reporter"
)

// ProgressWriter writes install progress updates to a reporter.
type ProgressWriter struct {
	// ongoingInstalls keeps track of ongoing install jobs per node.
	ongoingInstalls map[string]installJob
}

// UpdateJob updates the progress of a pull job for a given node and layer ID.
//
// It is supposed to be called whenever there is a progress update for a layer pull.
func (w *ProgressWriter) UpdateJob(node string, status *machine.LifecycleServiceInstallProgress) {
	if w.ongoingInstalls == nil {
		w.ongoingInstalls = make(map[string]installJob)
	}

	w.ongoingInstalls[node] = installJob{Status: status}
}

// PrintLayerProgress prints the current layer pull progress to the reporter.
func (w *ProgressWriter) PrintLayerProgress(rep *reporter.Reporter) {
	nodes := slices.Collect(maps.Keys(w.ongoingInstalls))
	sort.Strings(nodes)

	sb := strings.Builder{}

	for _, node := range nodes {
		sb.WriteString(node + ":\n")

		fmt.Fprintf(&sb, "  %s\n", w.ongoingInstalls[node].Status.Fmt())
	}

	rep.Report(reporter.Update{
		Message: sb.String(),
		Status:  reporter.StatusRunning,
	})
}

type installJob struct {
	Status *machine.LifecycleServiceInstallProgress
}
