// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"strconv"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/data"
)

// TalosInfo represents the Talos info widget.
type TalosInfo struct {
	tview.TextView
}

// NewTalosInfo initializes TalosInfo.
func NewTalosInfo() *TalosInfo {
	widget := &TalosInfo{
		TextView: *tview.NewTextView(),
	}

	widget.SetDynamicColors(true).
		SetText(noData).
		SetBorderPadding(1, 0, 1, 0)

	return widget
}

// Update implements the DataWidget interface.
func (widget *TalosInfo) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]
	if nodeData == nil {
		widget.SetText(noData)

		return
	}

	uuid := notAvailable
	cluster := notAvailable
	stage := notAvailable
	ready := notAvailable
	typ := notAvailable
	numMembers := notAvailable

	if nodeData.SystemInformation != nil {
		uuid = nodeData.SystemInformation.TypedSpec().UUID
	}

	if nodeData.ClusterInfo != nil && nodeData.ClusterInfo.TypedSpec().ClusterName != "" {
		cluster = nodeData.ClusterInfo.TypedSpec().ClusterName
	}

	if nodeData.MachineStatus != nil {
		stage = formatStatus(nodeData.MachineStatus.TypedSpec().Stage.String())
		ready = formatStatus(nodeData.MachineStatus.TypedSpec().Status.Ready)
	}

	if nodeData.MachineType != nil {
		typ = nodeData.MachineType.MachineType().String()
	}

	if len(nodeData.Members) > 0 {
		numMembers = strconv.Itoa(len(nodeData.Members))
	}

	fields := fieldGroup{
		fields: []field{
			{
				Name:  "UUID",
				Value: uuid,
			},
			{
				Name:  "CLUSTER",
				Value: cluster,
			},
			{
				Name:  "STAGE",
				Value: stage,
			},
			{
				Name:  "READY",
				Value: ready,
			},
			{
				Name:  "TYPE",
				Value: typ,
			},
			{
				Name:  "MEMBERS",
				Value: numMembers,
			},
		},
	}

	widget.SetText(fields.String())
}
