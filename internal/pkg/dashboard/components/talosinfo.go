// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

type talosInfoData struct {
	uuid            string
	clusterName     string
	siderolink      string
	stage           string
	ready           string
	numMachinesText string
	secureBootState string
	ephemeralMode   string

	stateVolumeIsMemory     bool
	ephemeralVolumeIsMemory bool
	stateVolumeKnown        bool
	ephemeralVolumeKnown    bool

	machineIDSet map[string]struct{}
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
	case *siderolink.Status:
		if data.Deleted {
			nodeData.siderolink = notAvailable
		} else {
			nodeData.siderolink = formatText(res.TypedSpec().Host, res.TypedSpec().Connected)
		}
	case *runtime.MachineStatus:
		if data.Deleted {
			nodeData.stage = notAvailable
			nodeData.ready = notAvailable
		} else {
			nodeData.stage = formatStatus(res.TypedSpec().Stage.String())
			nodeData.ready = formatStatus(res.TypedSpec().Status.Ready)
		}
	case *runtime.SecurityState:
		if data.Deleted {
			nodeData.secureBootState = notAvailable
		} else {
			nodeData.secureBootState = formatStatus(res.TypedSpec().SecureBoot)
		}
	case *cluster.Member:
		if data.Deleted {
			delete(nodeData.machineIDSet, res.Metadata().ID())
		} else {
			nodeData.machineIDSet[res.Metadata().ID()] = struct{}{}
		}

		suffix := ""
		if len(nodeData.machineIDSet) != 1 {
			suffix = "s"
		}

		nodeData.numMachinesText = fmt.Sprintf("(%d machine%s)", len(nodeData.machineIDSet), suffix)
	case *block.VolumeStatus:
		switch res.Metadata().ID() {
		case constants.StatePartitionLabel:
			nodeData.stateVolumeKnown = !data.Deleted
			nodeData.stateVolumeIsMemory = !data.Deleted && res.TypedSpec().Type == block.VolumeTypeMemory
		case constants.EphemeralPartitionLabel:
			nodeData.ephemeralVolumeKnown = !data.Deleted
			nodeData.ephemeralVolumeIsMemory = !data.Deleted && res.TypedSpec().Type == block.VolumeTypeMemory
		}

		nodeData.ephemeralMode = computeEphemeralMode(nodeData)
	}
}

func computeEphemeralMode(d *talosInfoData) string {
	if !d.stateVolumeKnown || !d.ephemeralVolumeKnown {
		return notAvailable
	}

	switch {
	case d.stateVolumeIsMemory && d.ephemeralVolumeIsMemory:
		return "yes (STATE + EPHEMERAL on tmpfs — state wiped on reboot)"
	case d.stateVolumeIsMemory:
		return "STATE-only (config wiped on reboot)"
	case d.ephemeralVolumeIsMemory:
		return "EPHEMERAL-only (runtime wiped on reboot)"
	default:
		return "no"
	}
}

func (widget *TalosInfo) getOrCreateNodeData(node string) *talosInfoData {
	nodeData, ok := widget.nodeMap[node]
	if !ok {
		nodeData = &talosInfoData{
			uuid:            notAvailable,
			clusterName:     notAvailable,
			siderolink:      notAvailable,
			stage:           notAvailable,
			ready:           notAvailable,
			numMachinesText: notAvailable,
			secureBootState: notAvailable,
			ephemeralMode:   notAvailable,
			machineIDSet:    make(map[string]struct{}),
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
				Value: data.clusterName + " " + data.numMachinesText,
			},
			{
				Name:  "SIDEROLINK",
				Value: data.siderolink,
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
				Name:  "SECUREBOOT",
				Value: data.secureBootState,
			},
			{
				Name:  "EPHEMERAL",
				Value: data.ephemeralMode,
			},
		},
	}

	widget.SetText(fields.String())
}
