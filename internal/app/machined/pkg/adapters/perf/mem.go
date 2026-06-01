// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"github.com/prometheus/procfs"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/resources/perf"
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
		MemTotal:          pointer.SafeDeref(info.MemTotal),
		MemUsed:           pointer.SafeDeref(info.MemTotal) - pointer.SafeDeref(info.MemFree),
		MemAvailable:      pointer.SafeDeref(info.MemAvailable),
		Buffers:           pointer.SafeDeref(info.Buffers),
		Cached:            pointer.SafeDeref(info.Cached),
		SwapCached:        pointer.SafeDeref(info.SwapCached),
		Active:            pointer.SafeDeref(info.Active),
		Inactive:          pointer.SafeDeref(info.Inactive),
		ActiveAnon:        pointer.SafeDeref(info.ActiveAnon),
		InactiveAnon:      pointer.SafeDeref(info.InactiveAnon),
		ActiveFile:        pointer.SafeDeref(info.ActiveFile),
		InactiveFile:      pointer.SafeDeref(info.InactiveFile),
		Unevictable:       pointer.SafeDeref(info.Unevictable),
		Mlocked:           pointer.SafeDeref(info.Mlocked),
		SwapTotal:         pointer.SafeDeref(info.SwapTotal),
		SwapFree:          pointer.SafeDeref(info.SwapFree),
		Dirty:             pointer.SafeDeref(info.Dirty),
		Writeback:         pointer.SafeDeref(info.Writeback),
		AnonPages:         pointer.SafeDeref(info.AnonPages),
		Mapped:            pointer.SafeDeref(info.Mapped),
		Shmem:             pointer.SafeDeref(info.Shmem),
		Slab:              pointer.SafeDeref(info.Slab),
		SReclaimable:      pointer.SafeDeref(info.SReclaimable),
		SUnreclaim:        pointer.SafeDeref(info.SUnreclaim),
		KernelStack:       pointer.SafeDeref(info.KernelStack),
		PageTables:        pointer.SafeDeref(info.PageTables),
		NFSunstable:       pointer.SafeDeref(info.NFSUnstable),
		Bounce:            pointer.SafeDeref(info.Bounce),
		WritebackTmp:      pointer.SafeDeref(info.WritebackTmp),
		CommitLimit:       pointer.SafeDeref(info.CommitLimit),
		CommittedAS:       pointer.SafeDeref(info.CommittedAS),
		VmallocTotal:      pointer.SafeDeref(info.VmallocTotal),
		VmallocUsed:       pointer.SafeDeref(info.VmallocUsed),
		VmallocChunk:      pointer.SafeDeref(info.VmallocChunk),
		HardwareCorrupted: pointer.SafeDeref(info.HardwareCorrupted),
		AnonHugePages:     pointer.SafeDeref(info.AnonHugePages),
		ShmemHugePages:    pointer.SafeDeref(info.ShmemHugePages),
		ShmemPmdMapped:    pointer.SafeDeref(info.ShmemPmdMapped),
		CmaTotal:          pointer.SafeDeref(info.CmaTotal),
		CmaFree:           pointer.SafeDeref(info.CmaFree),
		HugePagesTotal:    pointer.SafeDeref(info.HugePagesTotal),
		HugePagesFree:     pointer.SafeDeref(info.HugePagesFree),
		HugePagesRsvd:     pointer.SafeDeref(info.HugePagesRsvd),
		HugePagesSurp:     pointer.SafeDeref(info.HugePagesSurp),
		Hugepagesize:      pointer.SafeDeref(info.Hugepagesize),
		DirectMap4k:       pointer.SafeDeref(info.DirectMap4k),
		DirectMap2m:       pointer.SafeDeref(info.DirectMap2M),
		DirectMap1g:       pointer.SafeDeref(info.DirectMap1G),
	}
}
