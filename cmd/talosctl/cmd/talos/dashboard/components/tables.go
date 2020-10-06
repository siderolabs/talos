// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
)

// ProcessTable represents the widget with process info.
type ProcessTable struct {
	widgets.List
}

// NewProcessTable initializes ProcessTable.
func NewProcessTable() *ProcessTable {
	widget := &ProcessTable{
		List: *widgets.NewList(),
	}

	widget.Border = false
	widget.Title = fmt.Sprintf("%6s  %1s  %6s  %6s  %8s  %8s  %10s  %4s  %s",
		"PID",
		"S",
		"CPU%",
		"MEM%",
		"VIRT",
		"RES",
		"TIME+",
		"THR",
		"COMMAND",
	)
	widget.Rows = []string{
		noData,
	}
	widget.SelectedRowStyle = ui.NewStyle(ui.Theme.List.Text.Fg, ui.Theme.List.Text.Bg, ui.ModifierReverse)

	return widget
}

// Update implements the DataWidget interface.
func (widget *ProcessTable) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.Rows = []string{
			noData,
		}
	} else {
		widget.Rows = widget.Rows[:0]

		totalMem := nodeData.Memory.GetMeminfo().GetMemtotal() * 1024
		if totalMem == 0 {
			totalMem = 1
		}

		totalWeightedCPU := nodeData.CPUUsageByName("total_weighted")
		if totalWeightedCPU == 0 {
			totalWeightedCPU = 1
		}

		if nodeData.ProcsDiff != nil {
			sort.Slice(nodeData.Processes.Processes, func(i, j int) bool {
				proc1 := nodeData.Processes.Processes[i]
				proc2 := nodeData.Processes.Processes[j]

				return nodeData.ProcsDiff[proc1.Pid].CpuTime > nodeData.ProcsDiff[proc2.Pid].CpuTime
			})
		}

		for _, proc := range nodeData.Processes.Processes {
			var args string

			switch {
			case proc.Executable == "":
				args = proc.Command
			case proc.Args != "" && strings.Fields(proc.Args)[0] == filepath.Base(strings.Fields(proc.Executable)[0]):
				args = strings.Replace(proc.Args, strings.Fields(proc.Args)[0], proc.Executable, 1)
			default:
				args = proc.Args
			}

			line := fmt.Sprintf("%7d  %s  %6.1f  %6.1f  %8s  %8s  %10s  %4d  %s",
				proc.GetPid(),
				proc.State,
				nodeData.ProcsDiff[proc.Pid].CpuTime/totalWeightedCPU*100.0,
				float64(proc.ResidentMemory)/float64(totalMem)*100.0,
				humanize.Bytes(proc.VirtualMemory),
				humanize.Bytes(proc.ResidentMemory),
				time.Duration(proc.CpuTime)*time.Second,
				proc.Threads,
				args,
			)
			widget.Rows = append(widget.Rows, line)
		}
	}
}
