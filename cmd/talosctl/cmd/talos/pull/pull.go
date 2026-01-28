// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pull implements image pull progress reporting.
package pull

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/reporter"
)

// ProgressWriter writes pull progress updates to a reporter.
type ProgressWriter struct {
	// ongoingPulls keeps track of ongoing pull jobs per node.
	ongoingPulls map[string]pullJobs
}

// UpdateJob updates the progress of a pull job for a given node and layer ID.
//
// It is supposed to be called whenever there is a progress update for a layer pull.
func (w *ProgressWriter) UpdateJob(node, layerID string, progress *machine.ImageServicePullLayerProgress) {
	if w.ongoingPulls == nil {
		w.ongoingPulls = make(map[string]pullJobs)
	}

	ongoingPulls, ok := w.ongoingPulls[node]
	if !ok {
		ongoingPulls = pullJobs{}
	}

	for _, job := range ongoingPulls {
		if job.LayerID == layerID {
			job.Status = progress

			return
		}
	}

	ongoingPulls = append(ongoingPulls, &pullJob{
		LayerID: layerID,
		Status:  progress,
	})

	w.ongoingPulls[node] = ongoingPulls
}

// PrintLayerProgress prints the current layer pull progress to the reporter.
func (w *ProgressWriter) PrintLayerProgress(rep *reporter.Reporter) {
	nodes := slices.Collect(maps.Keys(w.ongoingPulls))
	sort.Strings(nodes)

	sb := strings.Builder{}

	for _, node := range nodes {
		sb.WriteString(node + ":\n")

		ongoingPulls := w.ongoingPulls[node]

		slices.SortFunc(ongoingPulls, func(a, b *pullJob) int {
			return strings.Compare(a.LayerID, b.LayerID)
		})

		for _, job := range ongoingPulls {
			fmt.Fprintf(&sb, "  %s\n", job.Status.Fmt())
		}
	}

	rep.Report(reporter.Update{
		Message: sb.String(),
		Status:  reporter.StatusRunning,
	})
}

type pullJob struct {
	LayerID string
	Status  *machine.ImageServicePullLayerProgress
}

type pullJobs []*pullJob

func (p pullJobs) Len() int {
	return len(p)
}

func (p pullJobs) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p pullJobs) Less(i, j int) bool {
	return p[i].LayerID < p[j].LayerID
}
