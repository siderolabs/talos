// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"math"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
)

// Header represents the top bar with host info.
type Header struct {
	tview.TextView
}

// NewHeader initializes Header.
func NewHeader() *Header {
	header := &Header{
		TextView: *tview.NewTextView(),
	}

	header.SetDynamicColors(true).SetText(noData)

	return header
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *Header) OnAPIDataChange(node string, data *apidata.Data) {
	nodeData := data.Nodes[node]
	_ = nodeData

	if nodeData == nil {
		widget.SetText(notAvailable)

		return
	}

	hostname := notAvailable
	uptime := notAvailable
	version := notAvailable
	numCPUs := notAvailable
	cpuFreq := notAvailable
	totalMem := notAvailable
	numProcesses := notAvailable

	cpuUsagePercent := fmt.Sprintf("%.1f%%", nodeData.CPUUsageByName("usage")*100.0)
	memUsagePercent := fmt.Sprintf("%.1f%%", nodeData.MemUsage()*100.0)

	if nodeData.Hostname != nil {
		hostname = nodeData.Hostname.GetHostname()
	}

	if nodeData.Version != nil {
		version = nodeData.Version.GetVersion().GetTag()
	}

	if nodeData.SystemStat != nil {
		uptime = time.Since(time.Unix(int64(nodeData.SystemStat.GetBootTime()), 0)).Round(time.Second).String()
	}

	if nodeData.CPUsInfo != nil {
		numCPUs = fmt.Sprintf("%d", len(nodeData.CPUsInfo.GetCpuInfo()))
		cpuFreq = widget.humanizeCPUFrequency(nodeData.CPUsInfo.GetCpuInfo()[0].GetCpuMhz())
	}

	if nodeData.Processes != nil {
		numProcesses = fmt.Sprintf("%d", len(nodeData.Processes.GetProcesses()))
	}

	if nodeData.Memory != nil {
		totalMem = humanize.IBytes(nodeData.Memory.GetMeminfo().GetMemtotal() << 10)
	}

	text := fmt.Sprintf(
		"[yellow::b]%s[-:-:-] (%s): uptime %s, %sx%s, %s RAM, PROCS %s, CPU %s, RAM %s",
		hostname,
		version,
		uptime,
		numCPUs,
		cpuFreq,
		totalMem,
		numProcesses,
		cpuUsagePercent,
		memUsagePercent,
	)

	widget.SetText(text)
}

func (widget *Header) humanizeCPUFrequency(mhz float64) string {
	value := math.Round(mhz)
	unit := "MHz"

	if mhz >= 1000 {
		ghz := value / 1000
		value = math.Round(ghz*100) / 100
		unit = "GHz"
	}

	return fmt.Sprintf("%s%s", humanize.Ftoa(value), unit)
}
