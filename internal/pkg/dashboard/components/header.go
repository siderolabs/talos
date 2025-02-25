// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const noHostname = "(no hostname)"

type headerData struct {
	hostname        string
	version         string
	uptime          string
	cpuFreq         string
	totalMem        string
	numProcesses    string
	cpuUsagePercent string
	memUsagePercent string
}

// Header represents the top bar with host info.
type Header struct {
	tview.TextView

	selectedNode string
	nodeMap      map[string]*headerData
}

// NewHeader initializes Header.
func NewHeader() *Header {
	header := &Header{
		TextView: *tview.NewTextView(),
		nodeMap:  make(map[string]*headerData),
	}

	header.SetDynamicColors(true).SetText(noData)

	return header
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *Header) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.redraw()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *Header) OnResourceDataChange(data resourcedata.Data) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	switch res := data.Resource.(type) { //nolint:gocritic
	case *network.HostnameStatus:
		if data.Deleted {
			nodeData.hostname = noHostname
		} else {
			nodeData.hostname = res.TypedSpec().Hostname
		}
	}

	if data.Node == widget.selectedNode {
		widget.redraw()
	}
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *Header) OnAPIDataChange(node string, data *apidata.Data) {
	for node, nodeData := range data.Nodes {
		widget.updateNodeAPIData(node, nodeData)
	}

	if node == widget.selectedNode {
		widget.redraw()
	}
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

func (widget *Header) redraw() {
	data := widget.getOrCreateNodeData(widget.selectedNode)

	text := fmt.Sprintf(
		"[yellow::b]%s[-:-:-] (%s): uptime %s, %s, %s RAM, PROCS %s, CPU %s, RAM %s",
		data.hostname,
		data.version,
		data.uptime,
		data.cpuFreq,
		data.totalMem,
		data.numProcesses,
		data.cpuUsagePercent,
		data.memUsagePercent,
	)

	widget.SetText(text)
}

//nolint:gocyclo
func (widget *Header) updateNodeAPIData(node string, data *apidata.Node) {
	nodeData := widget.getOrCreateNodeData(node)

	if data == nil {
		return
	}

	nodeData.cpuUsagePercent = fmt.Sprintf("%.1f%%", data.CPUUsageByName("usage")*100.0)
	nodeData.memUsagePercent = fmt.Sprintf("%.1f%%", data.MemUsage()*100.0)

	if data.Version != nil {
		nodeData.version = data.Version.GetVersion().GetTag()
	} else {
		nodeData.version = notAvailable
	}

	if data.SystemStat != nil && data.SystemStat.BootTime != 0 {
		nodeData.uptime = time.Since(time.Unix(int64(data.SystemStat.GetBootTime()), 0)).Round(time.Second).String()
	} else {
		nodeData.uptime = notAvailable
	}

	if data.CPUsInfo != nil {
		numCPUs := len(data.CPUsInfo.GetCpuInfo())

		if numCPUs > 0 {
			nodeData.cpuFreq = fmt.Sprintf("%dx%s", numCPUs, widget.humanizeCPUFrequency(data.CPUsInfo.GetCpuInfo()[0].GetCpuMhz()))
		}
	} else {
		nodeData.cpuFreq = notAvailable
	}

	if data.CPUsFreqStats != nil && data.CPUsFreqStats.CpuFreqStats != nil {
		numCPUs := len(data.CPUsFreqStats.CpuFreqStats)
		uniqMhz := make(map[uint64]int, numCPUs)

		for _, cpuFreqStat := range data.CPUsFreqStats.CpuFreqStats {
			uniqMhz[cpuFreqStat.CurrentFrequency]++
		}

		keys := make([]uint64, 0, len(uniqMhz))

		for mhz := range uniqMhz {
			if mhz == 0 {
				continue
			}

			keys = append(keys, mhz)
		}

		if len(keys) > 0 {
			sort.Slice(keys, func(i, j int) bool { return keys[i] > keys[j] })

			nodeData.cpuFreq = ""
		}

		for i, mhz := range keys {
			if i > 0 {
				nodeData.cpuFreq += " "
			}

			nodeData.cpuFreq += fmt.Sprintf("%dx%s", uniqMhz[mhz], widget.humanizeCPUFrequency(float64(mhz)/1000.0))
		}
	} else {
		nodeData.cpuFreq = notAvailable
	}

	if data.Processes != nil {
		nodeData.numProcesses = strconv.Itoa(len(data.Processes.GetProcesses()))
	} else {
		nodeData.numProcesses = notAvailable
	}

	if data.Memory != nil {
		nodeData.totalMem = humanize.IBytes(data.Memory.GetMeminfo().GetMemtotal() << 10)
	} else {
		nodeData.totalMem = notAvailable
	}
}

func (widget *Header) getOrCreateNodeData(node string) *headerData {
	data, ok := widget.nodeMap[node]
	if !ok {
		data = &headerData{
			hostname:        notAvailable,
			version:         notAvailable,
			uptime:          notAvailable,
			cpuFreq:         notAvailable,
			totalMem:        notAvailable,
			numProcesses:    notAvailable,
			cpuUsagePercent: notAvailable,
			memUsagePercent: notAvailable,
		}

		widget.nodeMap[node] = data
	}

	return data
}
