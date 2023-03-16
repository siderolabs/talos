// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apidata

import "github.com/siderolabs/talos/pkg/machinery/api/machine"

func cpuInfoDiff(old, next *machine.CPUStat) *machine.CPUStat {
	if old == nil || next == nil {
		return &machine.CPUStat{}
	}

	// TODO: support wraparound
	return &machine.CPUStat{
		User:      next.User - old.User,
		Nice:      next.Nice - old.Nice,
		System:    next.System - old.System,
		Idle:      next.Idle - old.Idle,
		Iowait:    next.Iowait - old.Iowait,
		Irq:       next.Irq - old.Irq,
		SoftIrq:   next.SoftIrq - old.SoftIrq,
		Steal:     next.Steal - old.Steal,
		Guest:     next.Guest - old.Guest,
		GuestNice: next.GuestNice - old.GuestNice,
	}
}

func netDevDiff(old, next *machine.NetDev) *machine.NetDev {
	if old == nil || next == nil {
		return &machine.NetDev{}
	}

	// TODO: support wraparound
	return &machine.NetDev{
		Name:         next.Name,
		RxBytes:      next.RxBytes - old.RxBytes,
		RxPackets:    next.RxPackets - old.RxPackets,
		RxErrors:     next.RxErrors - old.RxErrors,
		RxDropped:    next.RxDropped - old.RxDropped,
		RxFifo:       next.RxFifo - old.RxFifo,
		RxFrame:      next.RxFrame - old.RxFrame,
		RxCompressed: next.RxCompressed - old.RxCompressed,
		RxMulticast:  next.RxMulticast - old.RxMulticast,
		TxBytes:      next.TxBytes - old.TxBytes,
		TxPackets:    next.TxPackets - old.TxPackets,
		TxErrors:     next.TxErrors - old.TxErrors,
		TxDropped:    next.TxDropped - old.TxDropped,
		TxFifo:       next.TxFifo - old.TxFifo,
		TxCollisions: next.TxCollisions - old.TxCollisions,
		TxCarrier:    next.TxCarrier - old.TxCarrier,
		TxCompressed: next.TxCompressed - old.TxCompressed,
	}
}

func diskStatDiff(old, next *machine.DiskStat) *machine.DiskStat {
	if old == nil || next == nil {
		return &machine.DiskStat{}
	}

	// TODO: support wraparound
	return &machine.DiskStat{
		Name:             next.Name,
		ReadCompleted:    next.ReadCompleted - old.ReadCompleted,
		ReadMerged:       next.ReadMerged - old.ReadMerged,
		ReadSectors:      next.ReadSectors - old.ReadSectors,
		WriteCompleted:   next.WriteCompleted - old.WriteCompleted,
		WriteMerged:      next.WriteMerged - old.WriteMerged,
		WriteSectors:     next.WriteSectors - old.WriteSectors,
		DiscardCompleted: next.DiscardCompleted - old.DiscardCompleted,
		DiscardMerged:    next.DiscardMerged - old.DiscardMerged,
		DiscardSectors:   next.DiscardSectors - old.DiscardSectors,
	}
}

func procDiff(old, next *machine.ProcessInfo) *machine.ProcessInfo {
	if old == nil || next == nil {
		return &machine.ProcessInfo{}
	}

	// TODO: support wraparound
	return &machine.ProcessInfo{
		CpuTime: next.CpuTime - old.CpuTime,
	}
}
