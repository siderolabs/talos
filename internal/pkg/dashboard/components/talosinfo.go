// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"strconv"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type talosInfoData struct {
	uuid           string
	clusterName    string
	stage          string
	ready          string
	typ            string
	numMembersText string

	numMembers int
}

// TalosInfo represents the Talos info widget.
type TalosInfo struct {
	tview.TextView

	selectedNode string
	nodeMap      map[string]*talosInfoData
}

// NewTalosInfo initializes TalosInfo.
func NewTalosInfo() *TalosInfo {
	widget := &TalosInfo{
		TextView: *tview.NewTextView(),
		nodeMap:  make(map[string]*talosInfoData),
	}

	widget.SetDynamicColors(true).
		SetText(noData).
		SetBorderPadding(1, 0, 1, 0)

	return widget
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *TalosInfo) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.redraw()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *TalosInfo) OnResourceDataChange(data resourcedata.Data) {
	widget.updateNodeData(data)

	if data.Node == widget.selectedNode {
		widget.redraw()
	}
}

//nolint:gocyclo
func (widget *TalosInfo) updateNodeData(data resourcedata.Data) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	switch res := data.Resource.(type) {
	case *hardware.SystemInformation:
		if data.Deleted {
			nodeData.uuid = notAvailable
		} else {
			nodeData.uuid = res.TypedSpec().UUID
		}
	case *cluster.Info:
		clusterName := res.TypedSpec().ClusterName
		if data.Deleted || clusterName == "" {
			nodeData.clusterName = notAvailable
		} else {
			nodeData.clusterName = clusterName
		}
	case *runtime.MachineStatus:
		if data.Deleted {
			nodeData.stage = notAvailable
			nodeData.ready = notAvailable
		} else {
			nodeData.stage = formatStatus(res.TypedSpec().Stage.String())
			nodeData.ready = formatStatus(res.TypedSpec().Status.Ready)
		}
	case *config.MachineType:
		if data.Deleted {
			nodeData.typ = notAvailable
		} else {
			nodeData.typ = res.MachineType().String()
		}
	case *cluster.Member:
		if data.Deleted {
			nodeData.numMembers--
		} else {
			nodeData.numMembers++
		}

		nodeData.numMembersText = strconv.Itoa(nodeData.numMembers)
	}
}

func (widget *TalosInfo) getOrCreateNodeData(node string) *talosInfoData {
	nodeData, ok := widget.nodeMap[node]
	if !ok {
		nodeData = &talosInfoData{
			uuid:           notAvailable,
			clusterName:    notAvailable,
			stage:          notAvailable,
			ready:          notAvailable,
			typ:            notAvailable,
			numMembersText: notAvailable,
		}

		widget.nodeMap[node] = nodeData
	}

	return nodeData
}

func (widget *TalosInfo) redraw() {
	data := widget.getOrCreateNodeData(widget.selectedNode)

	fields := fieldGroup{
		fields: []field{
			{
				Name:  "UUID",
				Value: data.uuid,
			},
			{
				Name:  "CLUSTER",
				Value: data.clusterName,
			},
			{
				Name:  "STAGE",
				Value: data.stage,
			},
			{
				Name:  "READY",
				Value: data.ready,
			},
			{
				Name:  "TYPE",
				Value: data.typ,
			},
			{
				Name:  "MEMBERS",
				Value: data.numMembersText,
			},
		},
	}

	widget.SetText(fields.String())
}
