// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package apidata implements types to handle monitoring data, calculate values from it, etc.
package apidata

import (
	"time"
)

const maxPoints = 1000

// Data represents the monitoring data retrieved via Talos API.
//
// Data structure is sent over the channel each interval.
type Data struct {
	// Data per each node.
	Nodes map[string]*Node

	Timestamp time.Time
	Interval  time.Duration
}

// CalculateDiff with data from previous iteration.
func (data *Data) CalculateDiff(oldData *Data) {
	data.Interval = data.Timestamp.Sub(oldData.Timestamp)

	for node, nodeData := range data.Nodes {
		oldNodeData := oldData.Nodes[node]
		if oldNodeData == nil {
			continue
		}

		nodeData.UpdateDiff(oldNodeData)
		nodeData.UpdateSeries(oldNodeData)
	}
}
