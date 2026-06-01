// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
)

// LoadAvgInfo represents the widget with load average info.
type LoadAvgInfo struct {
	tview.TextView
}

// NewLoadAvgInfo initializes LoadAvgInfo.
func NewLoadAvgInfo() *LoadAvgInfo {
	widget := &LoadAvgInfo{
		TextView: *tview.NewTextView(),
	}

	widget.SetBorder(false)
	widget.SetBorderPadding(1, 0, 1, 0)
	widget.SetDynamicColors(true)

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *LoadAvgInfo) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.SetText(noData)
	} else {
		widget.SetText(fmt.Sprintf(
			"[::b]LOAD[::-]\n"+
				"1 min  [::b]%6.2f[::-]\n"+
				"5 min  [::b]%6.2f[::-]\n"+
				"15 min [::b]%6.2f[::-]",
			nodeData.LoadAvg.GetLoad1(),
			nodeData.LoadAvg.GetLoad5(),
			nodeData.LoadAvg.GetLoad15(),
		))
	}
}

// ProcsInfo represents the widget with processes info.
type ProcsInfo struct {
	tview.TextView
}

// NewProcsInfo initializes ProcsInfo.
func NewProcsInfo() *ProcsInfo {
	widget := &ProcsInfo{
		TextView: *tview.NewTextView(),
	}

	widget.SetBorder(false)
	widget.SetBorderPadding(1, 0, 1, 0)
	widget.SetDynamicColors(true)

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *ProcsInfo) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.SetText(noData)
	} else {
		procsCreated, suffix := humanize.ComputeSI(float64(nodeData.ProcsCreated()))

		widget.SetText(fmt.Sprintf(
			"[::b]PROCS[::-]\n"+
				"Created [::b]%5.1f%s[::-]\n"+
				"Running [::b]%5d[::-]\n"+
				"Blocked [::b]%5d[::-]",
			procsCreated, suffix,
			nodeData.SystemStat.GetProcessRunning(),
			nodeData.SystemStat.GetProcessBlocked(),
		))
	}
}

// MemInfo represents the widget with memory info.
type MemInfo struct {
	tview.TextView
}

// NewMemInfo initializes LoadAvgInfo.
func NewMemInfo() *MemInfo {
	widget := &MemInfo{
		TextView: *tview.NewTextView(),
	}

	widget.SetBorder(false)
	widget.SetBorderPadding(1, 0, 1, 0)
	widget.SetDynamicColors(true)

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *MemInfo) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.SetText(noData)
	} else {
		widget.SetText(fmt.Sprintf(
			"[::b]MEMORY[::-]\n"+
				"Total  [::b]%8s[::-]  Buffers [::b]%8s[::-]\n"+
				"Used   [::b]%8s[::-]  Cache   [::b]%8s[::-]\n"+
				"Free   [::b]%8s[::-]  Avail   [::b]%8s[::-]\n"+
				"Shared [::b]%8s[::-]  Swapped [::b]%8s[::-]\n",
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetMemtotal()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetBuffers()<<10),
			humanize.Bytes((nodeData.Memory.GetMeminfo().GetMemtotal()-nodeData.Memory.GetMeminfo().GetMemfree()-nodeData.Memory.GetMeminfo().GetCached()-nodeData.Memory.GetMeminfo().GetBuffers())<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetCached()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetMemfree()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetMemavailable()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetShmem()<<10),
			humanize.Bytes((nodeData.Memory.GetMeminfo().GetSwaptotal()-nodeData.Memory.GetMeminfo().GetSwapfree())<<10),
		))
	}
}

// CPUInfo represents the widget with CPU info.
type CPUInfo struct {
	tview.TextView
}

// NewCPUInfo initializes CPUInfo.
func NewCPUInfo() *CPUInfo {
	widget := &CPUInfo{
		TextView: *tview.NewTextView(),
	}

	widget.SetBorder(false)
	widget.SetBorderPadding(1, 0, 1, 0)
	widget.SetDynamicColors(true)

	return widget
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *CPUInfo) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.SetText(noData)
	} else {
		ctxSw, suffix := humanize.ComputeSI(float64(nodeData.CtxSwitches()))

		widget.SetText(fmt.Sprintf(
			"[::b]CPU[::-]\n"+
				"User   [::b]%5.1f%%[::-]  Nice   [::b]%5.1f%%[::-]\n"+
				"System [::b]%5.1f%%[::-]  IRQ    [::b]%5.1f%%[::-]\n"+
				"Idle   [::b]%5.1f%%[::-]  Iowait [::b]%5.1f%%[::-]\n"+
				"Steal  [::b]%5.1f%%[::-]  CtxSw  [::b]%5.1f%s[::-]\n",
			nodeData.CPUUsageByName("user")*100.0, nodeData.CPUUsageByName("nice")*100.0,
			nodeData.CPUUsageByName("system")*100.0, nodeData.CPUUsageByName("irq")*100.0,
			nodeData.CPUUsageByName("idle")*100.0, nodeData.CPUUsageByName("iowait")*100.0,
			nodeData.CPUUsageByName("steal")*100.0, ctxSw, suffix,
		))
	}
}
