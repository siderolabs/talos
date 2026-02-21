// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
)

// ProcessTable represents the widget with process info.
type ProcessTable struct {
	*tview.Table

	lastNode string
}

// NewProcessTable initializes ProcessTable.
func NewProcessTable() *ProcessTable {
	widget := &ProcessTable{
		Table: tview.NewTable(),
	}

	widget.SetFixed(1, 0)
	widget.SetBorders(false)
	widget.SetSelectable(true, false)
	widget.SetBorderPadding(0, 0, 1, 0)
	widget.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	widget.clear()

	return widget
}

// width constants for ProcessTable columns.
const (
	pidWidth     = 7
	stateWidth   = 1
	cpuWidth     = 6
	memWidth     = 6
	virtWidth    = 8
	resWidth     = 8
	timeWidth    = 10
	threadsWidth = 4
)

func (widget *ProcessTable) clear() {
	widget.Clear()

	widget.SetCell(0, 0, &tview.TableCell{
		Text:          "[::b]PID",
		Align:         tview.AlignRight,
		MaxWidth:      pidWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 1, &tview.TableCell{
		Text:          "[::b]S",
		Align:         tview.AlignCenter,
		MaxWidth:      stateWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 2, &tview.TableCell{
		Text:          "[::b]CPU%",
		Align:         tview.AlignRight,
		MaxWidth:      cpuWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 3, &tview.TableCell{
		Text:          "[::b]MEM%",
		Align:         tview.AlignRight,
		MaxWidth:      memWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 4, &tview.TableCell{
		Text:          "[::b]VIRT",
		Align:         tview.AlignRight,
		MaxWidth:      virtWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 5, &tview.TableCell{
		Text:          "[::b]RES",
		Align:         tview.AlignRight,
		MaxWidth:      resWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 6, &tview.TableCell{
		Text:          "[::b]TIME+",
		Align:         tview.AlignRight,
		MaxWidth:      timeWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 7, &tview.TableCell{
		Text:          "[::b]THR",
		Align:         tview.AlignRight,
		MaxWidth:      threadsWidth,
		NotSelectable: true,
	})
	widget.SetCell(0, 8, &tview.TableCell{
		Text:          "[::b]COMMAND",
		Align:         tview.AlignLeft,
		NotSelectable: true,
		Expansion:     1,
	})

	widget.SetCell(1, 0, &tview.TableCell{
		Text:          noData,
		Align:         tview.AlignCenter,
		NotSelectable: true,
	})
}

// OnAPIDataChange implements the APIDataListener interface.
//
//nolint:gocyclo
func (widget *ProcessTable) OnAPIDataChange(node string, data *apidata.Data) {
	widget.clear()

	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.Select(1, 0)

		return
	}

	totalMem := nodeData.Memory.GetMeminfo().GetMemtotal() * 1024
	if totalMem == 0 {
		totalMem = 1
	}

	totalWeightedCPU := nodeData.CPUUsageByName("total_weighted")
	if totalWeightedCPU == 0 {
		totalWeightedCPU = 1
	}

	// All downstream logic relies on nodeData.Processes to be not nil
	// Putting a check here to reduce cyclomatic complexity
	if nodeData.Processes == nil {
		return
	}

	if nodeData.ProcsDiff != nil {
		sort.Slice(nodeData.Processes.Processes, func(i, j int) bool {
			proc1 := nodeData.Processes.Processes[i]
			proc2 := nodeData.Processes.Processes[j]

			return nodeData.ProcsDiff[proc1.Pid].CpuTime > nodeData.ProcsDiff[proc2.Pid].CpuTime
		})
	}

	for idx, proc := range nodeData.Processes.Processes {
		var args string

		switch {
		case proc.Executable == "":
			args = proc.Command
		case proc.Args != "" && strings.Fields(proc.Args)[0] == filepath.Base(strings.Fields(proc.Executable)[0]):
			args = strings.Replace(proc.Args, strings.Fields(proc.Args)[0], proc.Executable, 1)
		default:
			args = proc.Args
		}

		// filter out non-printable characters
		args = strings.Map(func(r rune) rune {
			if r < 32 || r > 126 {
				return ' '
			}

			return r
		}, args)

		widget.SetCell(idx+1, 0, &tview.TableCell{
			Text:     strconv.FormatInt(int64(proc.GetPid()), 10),
			Align:    tview.AlignRight,
			MaxWidth: pidWidth,
		})

		widget.SetCell(idx+1, 1, &tview.TableCell{
			Text:     proc.State,
			Align:    tview.AlignCenter,
			MaxWidth: stateWidth,
		})

		widget.SetCell(idx+1, 2, &tview.TableCell{
			Text:     fmt.Sprintf("%.1f", nodeData.ProcsDiff[proc.Pid].GetCpuTime()/totalWeightedCPU*100.0),
			Align:    tview.AlignRight,
			MaxWidth: cpuWidth,
		})

		widget.SetCell(idx+1, 3, &tview.TableCell{
			Text:     fmt.Sprintf("%.1f", float64(proc.ResidentMemory)/float64(totalMem)*100.0),
			Align:    tview.AlignRight,
			MaxWidth: memWidth,
		})

		widget.SetCell(idx+1, 4, &tview.TableCell{
			Text:     humanize.Bytes(proc.VirtualMemory),
			Align:    tview.AlignRight,
			MaxWidth: virtWidth,
		})

		widget.SetCell(idx+1, 5, &tview.TableCell{
			Text:     humanize.Bytes(proc.ResidentMemory),
			Align:    tview.AlignRight,
			MaxWidth: resWidth,
		})

		widget.SetCell(idx+1, 6, &tview.TableCell{
			Text:     (time.Duration(proc.CpuTime) * time.Second).String(),
			Align:    tview.AlignRight,
			MaxWidth: timeWidth,
		})

		widget.SetCell(idx+1, 7, &tview.TableCell{
			Text:     strconv.FormatInt(int64(proc.Threads), 10),
			Align:    tview.AlignRight,
			MaxWidth: threadsWidth,
		})

		widget.SetCell(idx+1, 8, &tview.TableCell{
			Text:  args,
			Align: tview.AlignLeft,
		})
	}

	selectedRow, _ := widget.GetSelection()
	if selectedRow > len(nodeData.Processes.Processes)+1 || widget.lastNode != node {
		widget.Select(1, 0)
	}

	widget.lastNode = node
}
