// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"github.com/AlekSi/pointer"
	"github.com/prometheus/procfs"

	"github.com/talos-systems/talos/pkg/resources/perf"
)

// Memory adapter provides conversion from procfs.
//
//nolint:revive,golint
func Memory(r *perf.Memory) memory {
	return memory{
		Memory: r,
	}
}

type memory struct {
	*perf.Memory
}

// Update current Mem snapshot.
func (a memory) Update(info *procfs.Meminfo) {
	*a.Memory.TypedSpec() = perf.MemorySpec{
		MemTotal:          pointer.GetUint64(info.MemTotal),
		MemUsed:           pointer.GetUint64(info.MemTotal) - pointer.GetUint64(info.MemFree),
		MemAvailable:      pointer.GetUint64(info.MemAvailable),
		Buffers:           pointer.GetUint64(info.Buffers),
		Cached:            pointer.GetUint64(info.Cached),
		SwapCached:        pointer.GetUint64(info.SwapCached),
		Active:            pointer.GetUint64(info.Active),
		Inactive:          pointer.GetUint64(info.Inactive),
		ActiveAnon:        pointer.GetUint64(info.ActiveAnon),
		InactiveAnon:      pointer.GetUint64(info.InactiveAnon),
		ActiveFile:        pointer.GetUint64(info.ActiveFile),
		InactiveFile:      pointer.GetUint64(info.InactiveFile),
		Unevictable:       pointer.GetUint64(info.Unevictable),
		Mlocked:           pointer.GetUint64(info.Mlocked),
		SwapTotal:         pointer.GetUint64(info.SwapTotal),
		SwapFree:          pointer.GetUint64(info.SwapFree),
		Dirty:             pointer.GetUint64(info.Dirty),
		Writeback:         pointer.GetUint64(info.Writeback),
		AnonPages:         pointer.GetUint64(info.AnonPages),
		Mapped:            pointer.GetUint64(info.Mapped),
		Shmem:             pointer.GetUint64(info.Shmem),
		Slab:              pointer.GetUint64(info.Slab),
		SReclaimable:      pointer.GetUint64(info.SReclaimable),
		SUnreclaim:        pointer.GetUint64(info.SUnreclaim),
		KernelStack:       pointer.GetUint64(info.KernelStack),
		PageTables:        pointer.GetUint64(info.PageTables),
		NFSunstable:       pointer.GetUint64(info.NFSUnstable),
		Bounce:            pointer.GetUint64(info.Bounce),
		WritebackTmp:      pointer.GetUint64(info.WritebackTmp),
		CommitLimit:       pointer.GetUint64(info.CommitLimit),
		CommittedAS:       pointer.GetUint64(info.CommittedAS),
		VmallocTotal:      pointer.GetUint64(info.VmallocTotal),
		VmallocUsed:       pointer.GetUint64(info.VmallocUsed),
		VmallocChunk:      pointer.GetUint64(info.VmallocChunk),
		HardwareCorrupted: pointer.GetUint64(info.HardwareCorrupted),
		AnonHugePages:     pointer.GetUint64(info.AnonHugePages),
		ShmemHugePages:    pointer.GetUint64(info.ShmemHugePages),
		ShmemPmdMapped:    pointer.GetUint64(info.ShmemPmdMapped),
		CmaTotal:          pointer.GetUint64(info.CmaTotal),
		CmaFree:           pointer.GetUint64(info.CmaFree),
		HugePagesTotal:    pointer.GetUint64(info.HugePagesTotal),
		HugePagesFree:     pointer.GetUint64(info.HugePagesFree),
		HugePagesRsvd:     pointer.GetUint64(info.HugePagesRsvd),
		HugePagesSurp:     pointer.GetUint64(info.HugePagesSurp),
		Hugepagesize:      pointer.GetUint64(info.Hugepagesize),
		DirectMap4k:       pointer.GetUint64(info.DirectMap4k),
		DirectMap2m:       pointer.GetUint64(info.DirectMap2M),
		DirectMap1g:       pointer.GetUint64(info.DirectMap1G),
	}
}
