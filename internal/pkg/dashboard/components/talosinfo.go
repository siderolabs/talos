// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type talosInfoData struct {
	uuid                          string
	clusterName                   string
	stage                         string
	ready                         string
	typ                           string
	numMachinesText               string
	secureBootState               string
	statePartitionMountStatus     string
	ephemeralPartitionMountStatus string

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
	case *runtime.MountStatus:
		switch res.Metadata().ID() {
		case constants.StatePartitionLabel:
			if data.Deleted {
				nodeData.statePartitionMountStatus = notAvailable
			} else {
				nodeData.statePartitionMountStatus = mountStatus(res.TypedSpec().Encrypted, res.TypedSpec().EncryptionProviders)
			}
		case constants.EphemeralPartitionLabel:
			if data.Deleted {
				nodeData.ephemeralPartitionMountStatus = notAvailable
			} else {
				nodeData.ephemeralPartitionMountStatus = mountStatus(res.TypedSpec().Encrypted, res.TypedSpec().EncryptionProviders)
			}
		}

	case *config.MachineType:
		if data.Deleted {
			nodeData.typ = notAvailable
		} else {
			nodeData.typ = res.MachineType().String()
		}
	case *cluster.Member:
		if data.Deleted {
			delete(nodeData.machineIDSet, res.Metadata().ID())
		} else {
			nodeData.machineIDSet[res.Metadata().ID()] = struct{}{}
		}

		nodeData.numMachinesText = strconv.Itoa(len(nodeData.machineIDSet))
	}
}

func (widget *TalosInfo) getOrCreateNodeData(node string) *talosInfoData {
	nodeData, ok := widget.nodeMap[node]
	if !ok {
		nodeData = &talosInfoData{
			uuid:                          notAvailable,
			clusterName:                   notAvailable,
			stage:                         notAvailable,
			ready:                         notAvailable,
			typ:                           notAvailable,
			numMachinesText:               notAvailable,
			secureBootState:               notAvailable,
			statePartitionMountStatus:     notAvailable,
			ephemeralPartitionMountStatus: notAvailable,
			machineIDSet:                  make(map[string]struct{}),
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
				Name:  "MACHINES",
				Value: data.numMachinesText,
			},
			{
				Name:  "SECUREBOOT",
				Value: data.secureBootState,
			},
			{
				Name:  "STATE",
				Value: data.statePartitionMountStatus,
			},
			{
				Name:  "EPHEMERAL",
				Value: data.ephemeralPartitionMountStatus,
			},
		},
	}

	widget.SetText(fields.String())
}

func mountStatus(encrypted bool, providers []string) string {
	if !encrypted {
		return "[green]OK[-]"
	}

	return fmt.Sprintf("[green]OK - encrypted[-] (%s)", strings.Join(providers, ","))
}
