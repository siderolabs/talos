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

	lastNode     string
	dataCells    [][processTableCols]*tview.TableCell
	lastRowCount int
}

// processTableCols is the number of columns in the ProcessTable.
const processTableCols = 9

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

	// Header cells are created once and never replaced.
	widget.SetCell(0, 0, &tview.TableCell{Text: "[::b]PID", Align: tview.AlignRight, MaxWidth: pidWidth, NotSelectable: true})
	widget.SetCell(0, 1, &tview.TableCell{Text: "[::b]S", Align: tview.AlignCenter, MaxWidth: stateWidth, NotSelectable: true})
	widget.SetCell(0, 2, &tview.TableCell{Text: "[::b]CPU%", Align: tview.AlignRight, MaxWidth: cpuWidth, NotSelectable: true})
	widget.SetCell(0, 3, &tview.TableCell{Text: "[::b]MEM%", Align: tview.AlignRight, MaxWidth: memWidth, NotSelectable: true})
	widget.SetCell(0, 4, &tview.TableCell{Text: "[::b]VIRT", Align: tview.AlignRight, MaxWidth: virtWidth, NotSelectable: true})
	widget.SetCell(0, 5, &tview.TableCell{Text: "[::b]RES", Align: tview.AlignRight, MaxWidth: resWidth, NotSelectable: true})
	widget.SetCell(0, 6, &tview.TableCell{Text: "[::b]TIME+", Align: tview.AlignRight, MaxWidth: timeWidth, NotSelectable: true})
	widget.SetCell(0, 7, &tview.TableCell{Text: "[::b]THR", Align: tview.AlignRight, MaxWidth: threadsWidth, NotSelectable: true})
	widget.SetCell(0, 8, &tview.TableCell{Text: "[::b]COMMAND", Align: tview.AlignLeft, NotSelectable: true, Expansion: 1})

	// Pre-allocate the noData placeholder in row 1.
	widget.ensureDataRows(1)
	widget.dataCells[0][0].Text = noData
	widget.dataCells[0][0].Align = tview.AlignCenter
	widget.lastRowCount = 0 // 0 means noData is displayed

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

// ensureDataRows grows the dataCells pool to hold at least n rows,
// calling SetCell for any newly allocated cells.
func (widget *ProcessTable) ensureDataRows(n int) {
	for len(widget.dataCells) < n {
		row := len(widget.dataCells)

		var cells [processTableCols]*tview.TableCell

		cells[0] = &tview.TableCell{Align: tview.AlignRight, MaxWidth: pidWidth}
		cells[1] = &tview.TableCell{Align: tview.AlignCenter, MaxWidth: stateWidth}
		cells[2] = &tview.TableCell{Align: tview.AlignRight, MaxWidth: cpuWidth}
		cells[3] = &tview.TableCell{Align: tview.AlignRight, MaxWidth: memWidth}
		cells[4] = &tview.TableCell{Align: tview.AlignRight, MaxWidth: virtWidth}
		cells[5] = &tview.TableCell{Align: tview.AlignRight, MaxWidth: resWidth}
		cells[6] = &tview.TableCell{Align: tview.AlignRight, MaxWidth: timeWidth}
		cells[7] = &tview.TableCell{Align: tview.AlignRight, MaxWidth: threadsWidth}
		cells[8] = &tview.TableCell{Align: tview.AlignLeft, Expansion: 1}

		widget.dataCells = append(widget.dataCells, cells)

		for col, cell := range cells {
			widget.SetCell(row+1, col, cell) // +1 because row 0 is the header
		}
	}
}

// OnAPIDataChange implements the APIDataListener interface.
//
//nolint:gocyclo
func (widget *ProcessTable) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil || nodeData.Processes == nil {
		// Show noData in first data row; blank any previously filled rows.
		widget.ensureDataRows(1)
		widget.dataCells[0][0].Text = noData
		widget.dataCells[0][0].Align = tview.AlignCenter

		for j := 1; j < processTableCols; j++ {
			widget.dataCells[0][j].Text = ""
		}

		for i := 1; i < widget.lastRowCount; i++ {
			for col := range widget.dataCells[i] {
				widget.dataCells[i][col].Text = ""
			}
		}

		widget.lastRowCount = 0
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

	if nodeData.ProcsDiff != nil {
		sort.Slice(nodeData.Processes.Processes, func(i, j int) bool {
			return nodeData.ProcsDiff[nodeData.Processes.Processes[i].Pid] > nodeData.ProcsDiff[nodeData.Processes.Processes[j].Pid]
		})
	}

	numProcs := len(nodeData.Processes.Processes)
	widget.ensureDataRows(numProcs)

	for idx, proc := range nodeData.Processes.Processes {
		cells := widget.dataCells[idx]

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

		// Restore normal alignment for the first cell (may have been used for noData).
		cells[0].Align = tview.AlignRight
		cells[0].Text = strconv.FormatInt(int64(proc.GetPid()), 10)
		cells[1].Text = proc.State
		cells[2].Text = fmt.Sprintf("%.1f", nodeData.ProcsDiff[proc.Pid]/totalWeightedCPU*100.0)
		cells[3].Text = fmt.Sprintf("%.1f", float64(proc.ResidentMemory)/float64(totalMem)*100.0)
		cells[4].Text = humanize.Bytes(proc.VirtualMemory)
		cells[5].Text = humanize.Bytes(proc.ResidentMemory)
		cells[6].Text = (time.Duration(proc.CpuTime) * time.Second).String()
		cells[7].Text = strconv.FormatInt(int64(proc.Threads), 10)
		cells[8].Text = args
	}

	// Blank rows that are no longer needed.
	for i := numProcs; i < widget.lastRowCount; i++ {
		for col := range widget.dataCells[i] {
			widget.dataCells[i][col].Text = ""
		}
	}

	widget.lastRowCount = numProcs

	selectedRow, _ := widget.GetSelection()
	if selectedRow > numProcs+1 || widget.lastNode != node {
		widget.Select(1, 0)
	}

	widget.lastNode = node
}
