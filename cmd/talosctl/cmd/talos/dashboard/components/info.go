// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/gizak/termui/v3/widgets"

	"github.com/talos-systems/talos/cmd/talosctl/cmd/talos/dashboard/data"
)

// LoadAvgInfo represents the widget with load average info.
type LoadAvgInfo struct {
	widgets.Paragraph
}

// NewLoadAvgInfo initializes LoadAvgInfo.
func NewLoadAvgInfo() *LoadAvgInfo {
	widget := &LoadAvgInfo{
		Paragraph: *widgets.NewParagraph(),
	}

	widget.Border = false
	widget.Title = "LOAD"
	widget.PaddingLeft = 1

	return widget
}

// Update implements the DataWidget interface.
func (widget *LoadAvgInfo) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.Text = noData
	} else {
		widget.Text = fmt.Sprintf(
			"1 min  [%6.2f](mod:bold)\n"+
				"5 min  [%6.2f](mod:bold)\n"+
				"15 min [%6.2f](mod:bold)",
			nodeData.LoadAvg.GetLoad1(),
			nodeData.LoadAvg.GetLoad5(),
			nodeData.LoadAvg.GetLoad15(),
		)
	}
}

// ProcsInfo represents the widget with processes info.
type ProcsInfo struct {
	widgets.Paragraph
}

// NewProcsInfo initializes ProcsInfo.
func NewProcsInfo() *ProcsInfo {
	widget := &ProcsInfo{
		Paragraph: *widgets.NewParagraph(),
	}

	widget.Border = false
	widget.Title = "PROCS"
	widget.PaddingLeft = 1

	return widget
}

// Update implements the DataWidget interface.
func (widget *ProcsInfo) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.Text = noData
	} else {
		procsCreated, suffix := humanize.ComputeSI(float64(nodeData.ProcsCreated()))

		widget.Text = fmt.Sprintf(
			"Created [%5.1f%s](mod:bold)\n"+
				"Running [%5d](mod:bold)\n"+
				"Blocked [%5d](mod:bold)",
			procsCreated, suffix,
			nodeData.SystemStat.GetProcessRunning(),
			nodeData.SystemStat.GetProcessBlocked(),
		)
	}
}

// MemInfo represents the widget with memory info.
type MemInfo struct {
	widgets.Paragraph
}

// NewMemInfo initializes LoadAvgInfo.
func NewMemInfo() *MemInfo {
	widget := &MemInfo{
		Paragraph: *widgets.NewParagraph(),
	}

	widget.Border = false
	widget.Title = "MEMORY"
	widget.PaddingLeft = 1

	return widget
}

// Update implements the DataWidget interface.
func (widget *MemInfo) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.Text = noData
	} else {
		widget.Text = fmt.Sprintf(
			"Total  [%8s](mod:bold)  Buffers [%8s](mod:bold)\n"+
				"Used   [%8s](mod:bold)  Cache   [%8s](mod:bold)\n"+
				"Free   [%8s](mod:bold)  Avail   [%8s](mod:bold)\n"+
				"Shared [%8s](mod:bold)\n",
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetMemtotal()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetBuffers()<<10),
			humanize.Bytes((nodeData.Memory.GetMeminfo().GetMemtotal()-nodeData.Memory.GetMeminfo().GetMemfree()-nodeData.Memory.GetMeminfo().GetCached()-nodeData.Memory.GetMeminfo().GetBuffers())<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetCached()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetMemfree()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetMemavailable()<<10),
			humanize.Bytes(nodeData.Memory.GetMeminfo().GetShmem()<<10),
		)
	}
}

// CPUInfo represents the widget with CPU info.
type CPUInfo struct {
	widgets.Paragraph
}

// NewCPUInfo initializes CPUInfo.
func NewCPUInfo() *CPUInfo {
	widget := &CPUInfo{
		Paragraph: *widgets.NewParagraph(),
	}

	widget.Border = false
	widget.Title = "CPU"
	widget.PaddingLeft = 1

	return widget
}

// Update implements the DataWidget interface.
func (widget *CPUInfo) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]

	if nodeData == nil {
		widget.Text = noData
	} else {
		ctxSw, suffix := humanize.ComputeSI(float64(nodeData.CtxSwitches()))

		widget.Text = fmt.Sprintf(
			"User   [%5.1f%%](mod:bold)  Nice   [%5.1f%%](mod:bold)\n"+
				"System [%5.1f%%](mod:bold)  IRQ    [%5.1f%%](mod:bold)\n"+
				"Idle   [%5.1f%%](mod:bold)  Iowait [%5.1f%%](mod:bold)\n"+
				"Steal  [%5.1f%%](mod:bold)  CtxSw  [%5.1f%s](mod:bold)\n",
			nodeData.CPUUsageByName("user")*100.0, nodeData.CPUUsageByName("nice")*100.0,
			nodeData.CPUUsageByName("system")*100.0, nodeData.CPUUsageByName("irq")*100.0,
			nodeData.CPUUsageByName("idle")*100.0, nodeData.CPUUsageByName("iowait")*100.0,
			nodeData.CPUUsageByName("steal")*100.0, ctxSw, suffix,
		)
	}
}
