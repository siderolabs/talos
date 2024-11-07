// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apidata

import (
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// Node represents data gathered from a single node.
type Node struct {
	// These fields are directly API responses.
	Hostname      *machine.Hostname
	LoadAvg       *machine.LoadAvg
	Version       *machine.Version
	Memory        *machine.Memory
	SystemStat    *machine.SystemStat
	CPUsFreqStats *machine.CPUsFreqStats
	CPUsInfo      *machine.CPUsInfo
	NetDevStats   *machine.NetworkDeviceStats
	DiskStats     *machine.DiskStats
	Processes     *machine.Process
	ServiceList   *machine.ServiceList

	// These fields are calculated as diff with Node data from previous pol.
	SystemStatDiff  *machine.SystemStat
	NetDevStatsDiff *machine.NetworkDeviceStats
	DiskStatsDiff   *machine.DiskStats
	ProcsDiff       map[int32]*machine.ProcessInfo

	// Time-series data.
	Series map[string][]float64
}

// MemUsage as used/total.
func (node *Node) MemUsage() float64 {
	memTotal := node.Memory.GetMeminfo().GetMemtotal()
	memUsed := node.Memory.GetMeminfo().GetMemtotal() - node.Memory.GetMeminfo().GetMemfree() - node.Memory.GetMeminfo().GetCached() - node.Memory.GetMeminfo().GetBuffers()

	if memTotal == 0 {
		return 0
	}

	return float64(memUsed) / float64(memTotal)
}

// CPUUsageByName returns CPU usage by name.
//
//nolint:gocyclo
func (node *Node) CPUUsageByName(name string) float64 {
	if node.SystemStatDiff == nil || node.SystemStatDiff.CpuTotal == nil {
		return 0
	}

	stat := node.SystemStatDiff.CpuTotal

	idle := stat.Idle + stat.Iowait
	nonIdle := stat.User + stat.Nice + stat.System + stat.Irq + stat.Steal + stat.SoftIrq
	total := idle + nonIdle

	if total == 0 {
		return 0
	}

	switch name {
	case "user":
		return stat.User / total
	case "system":
		return stat.System / total
	case "idle":
		return stat.Idle / total
	case "iowait":
		return stat.Iowait / total
	case "nice":
		return stat.Nice / total
	case "irq":
		return stat.Irq / total
	case "steal":
		return stat.Steal / total
	case "softirq":
		return stat.SoftIrq / total
	case "usage":
		return (total - idle) / total
	case "total":
		return total
	case "total_weighted":
		cpuCount := len(node.CPUsInfo.GetCpuInfo())
		if cpuCount == 0 {
			return total
		}

		return total / float64(cpuCount)
	}

	panic("unknown cpu usage name")
}

// CtxSwitches returns diff of context switches.
func (node *Node) CtxSwitches() uint64 {
	if node.SystemStatDiff == nil {
		return 0
	}

	return node.SystemStatDiff.GetContextSwitches()
}

// ProcsCreated returns diff of processes created.
func (node *Node) ProcsCreated() uint64 {
	if node.SystemStatDiff == nil {
		return 0
	}

	return node.SystemStatDiff.GetProcessCreated()
}

// UpdateSeries builds time-series data based on previous iteration data.
func (node *Node) UpdateSeries(old *Node) {
	node.Series = make(map[string][]float64)

	for _, graphInfo := range []struct {
		name string
		f    func() float64
	}{
		{
			"mem",
			node.MemUsage,
		},
		{
			"user",
			func() float64 { return node.CPUUsageByName("user") },
		},
		{
			"system",
			func() float64 { return node.CPUUsageByName("system") },
		},
		{
			"loadavg",
			func() float64 { return node.LoadAvg.GetLoad1() },
		},
		{
			"netrxbytes",
			func() float64 { return float64(node.NetDevStatsDiff.GetTotal().GetRxBytes()) },
		},
		{
			"nettxbytes",
			func() float64 { return float64(node.NetDevStatsDiff.GetTotal().GetTxBytes()) },
		},
		{
			"diskrdsectors",
			func() float64 { return float64(node.DiskStatsDiff.GetTotal().GetReadSectors()) },
		},
		{
			"diskwrsectors",
			func() float64 { return float64(node.DiskStatsDiff.GetTotal().GetWriteSectors()) },
		},
	} {
		oldSeries := old.Series[graphInfo.name]

		off := 0
		if len(oldSeries) > maxPoints {
			off = len(oldSeries) - maxPoints
		}

		node.Series[graphInfo.name] = append(oldSeries[off:], graphInfo.f())

		// TODO: bug with plot widget
		for len(node.Series[graphInfo.name]) < 2 {
			node.Series[graphInfo.name] = append([]float64{0.0}, node.Series[graphInfo.name]...)
		}
	}
}

// UpdateDiff calculates diff with node data from previous iteration.
func (node *Node) UpdateDiff(old *Node) {
	if old.SystemStat != nil {
		node.SystemStatDiff = &machine.SystemStat{
			// TODO: support other fields
			CpuTotal:        cpuInfoDiff(old.SystemStat.GetCpuTotal(), node.SystemStat.GetCpuTotal()),
			ContextSwitches: node.SystemStat.ContextSwitches - old.SystemStat.ContextSwitches,
			ProcessCreated:  node.SystemStat.ProcessCreated - old.SystemStat.ProcessCreated,
		}
	}

	if old.NetDevStats != nil {
		node.NetDevStatsDiff = &machine.NetworkDeviceStats{
			// TODO: support other fields
			Total: netDevDiff(old.NetDevStats.GetTotal(), node.NetDevStats.GetTotal()),
		}
	}

	if old.DiskStats != nil {
		node.DiskStatsDiff = &machine.DiskStats{
			// TODO: support other fields
			Total: diskStatDiff(old.DiskStats.GetTotal(), node.DiskStats.GetTotal()),
		}
	}

	if old.Processes != nil {
		index := xslices.ToMap(old.Processes.GetProcesses(), func(proc *machine.ProcessInfo) (int32, *machine.ProcessInfo) {
			return proc.Pid, proc
		})

		node.ProcsDiff = xslices.ToMap(node.Processes.GetProcesses(), func(proc *machine.ProcessInfo) (int32, *machine.ProcessInfo) {
			return proc.Pid, procDiff(index[proc.Pid], proc)
		})
	}
}
