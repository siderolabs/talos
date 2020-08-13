// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package data

import "github.com/talos-systems/talos/pkg/machinery/api/machine"

func cpuInfoDiff(old, new *machine.CPUStat) *machine.CPUStat {
	if old == nil || new == nil {
		return &machine.CPUStat{}
	}

	// TODO: support wraparound
	return &machine.CPUStat{
		User:      new.User - old.User,
		Nice:      new.Nice - old.Nice,
		System:    new.System - old.System,
		Idle:      new.Idle - old.Idle,
		Iowait:    new.Iowait - old.Iowait,
		Irq:       new.Irq - old.Irq,
		SoftIrq:   new.SoftIrq - old.SoftIrq,
		Steal:     new.Steal - old.Steal,
		Guest:     new.Guest - old.Guest,
		GuestNice: new.GuestNice - old.GuestNice,
	}
}

func netDevDiff(old, new *machine.NetDev) *machine.NetDev {
	if old == nil || new == nil {
		return &machine.NetDev{}
	}

	// TODO: support wraparound
	return &machine.NetDev{
		Name:         new.Name,
		RxBytes:      new.RxBytes - old.RxBytes,
		RxPackets:    new.RxPackets - old.RxPackets,
		RxErrors:     new.RxErrors - old.RxErrors,
		RxDropped:    new.RxDropped - old.RxDropped,
		RxFifo:       new.RxFifo - old.RxFifo,
		RxFrame:      new.RxFrame - old.RxFrame,
		RxCompressed: new.RxCompressed - old.RxCompressed,
		RxMulticast:  new.RxMulticast - old.RxMulticast,
		TxBytes:      new.TxBytes - old.TxBytes,
		TxPackets:    new.TxPackets - old.TxPackets,
		TxErrors:     new.TxErrors - old.TxErrors,
		TxDropped:    new.TxDropped - old.TxDropped,
		TxFifo:       new.TxFifo - old.TxFifo,
		TxCollisions: new.TxCollisions - old.TxCollisions,
		TxCarrier:    new.TxCarrier - old.TxCarrier,
		TxCompressed: new.TxCompressed - old.TxCompressed,
	}
}

func diskStatDiff(old, new *machine.DiskStat) *machine.DiskStat {
	if old == nil || new == nil {
		return &machine.DiskStat{}
	}

	// TODO: support wraparound
	return &machine.DiskStat{
		Name:             new.Name,
		ReadCompleted:    new.ReadCompleted - old.ReadCompleted,
		ReadMerged:       new.ReadMerged - old.ReadMerged,
		ReadSectors:      new.ReadSectors - old.ReadSectors,
		WriteCompleted:   new.WriteCompleted - old.WriteCompleted,
		WriteMerged:      new.WriteMerged - old.WriteMerged,
		WriteSectors:     new.WriteSectors - old.WriteSectors,
		DiscardCompleted: new.DiscardCompleted - old.DiscardCompleted,
		DiscardMerged:    new.DiscardMerged - old.DiscardMerged,
		DiscardSectors:   new.DiscardSectors - old.DiscardSectors,
	}
}

func procDiff(old, new *machine.ProcessInfo) *machine.ProcessInfo {
	if old == nil || new == nil {
		return &machine.ProcessInfo{}
	}

	// TODO: support wraparound
	return &machine.ProcessInfo{
		CpuTime: new.CpuTime - old.CpuTime,
	}
}
