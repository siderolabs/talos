// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/prometheus/procfs"
)

// MemoryType is type of Etcd resource.
const MemoryType = resource.Type("MemoryStats.perf.talos.dev")

// MemoryID is a resource ID of singleton instance.
const MemoryID = resource.ID("latest")

// Memory represents the last Memory stats snapshot.
type Memory struct {
	md   resource.Metadata
	spec MemorySpec
}

// MemorySpec represents the last Memory stats snapshot.
type MemorySpec struct {
	MemTotal          uint64 `yaml:"total"`
	MemUsed           uint64 `yaml:"used"`
	MemAvailable      uint64 `yaml:"available"`
	Buffers           uint64 `yaml:"buffers"`
	Cached            uint64 `yaml:"cached"`
	SwapCached        uint64 `yaml:"swapCached"`
	Active            uint64 `yaml:"active"`
	Inactive          uint64 `yaml:"inactive"`
	ActiveAnon        uint64 `yaml:"activeAnon"`
	InactiveAnon      uint64 `yaml:"inactiveAnon"`
	ActiveFile        uint64 `yaml:"activeFile"`
	InactiveFile      uint64 `yaml:"inactiveFile"`
	Unevictable       uint64 `yaml:"unevictable"`
	Mlocked           uint64 `yaml:"mlocked"`
	SwapTotal         uint64 `yaml:"swapTotal"`
	SwapFree          uint64 `yaml:"swapFree"`
	Dirty             uint64 `yaml:"dirty"`
	Writeback         uint64 `yaml:"writeback"`
	AnonPages         uint64 `yaml:"anonPages"`
	Mapped            uint64 `yaml:"mapped"`
	Shmem             uint64 `yaml:"shmem"`
	Slab              uint64 `yaml:"slab"`
	SReclaimable      uint64 `yaml:"sreclaimable"`
	SUnreclaim        uint64 `yaml:"sunreclaim"`
	KernelStack       uint64 `yaml:"kernelStack"`
	PageTables        uint64 `yaml:"pageTables"`
	NFSunstable       uint64 `yaml:"nfsunstable"`
	Bounce            uint64 `yaml:"bounce"`
	WritebackTmp      uint64 `yaml:"writeBacktmp"`
	CommitLimit       uint64 `yaml:"commitLimit"`
	CommittedAS       uint64 `yaml:"commitTedas"`
	VmallocTotal      uint64 `yaml:"vmallocTotal"`
	VmallocUsed       uint64 `yaml:"vmallocUsed"`
	VmallocChunk      uint64 `yaml:"vmallocChunk"`
	HardwareCorrupted uint64 `yaml:"hardwareCorrupted"`
	AnonHugePages     uint64 `yaml:"anonHugePages"`
	ShmemHugePages    uint64 `yaml:"shmemHugePages"`
	ShmemPmdMapped    uint64 `yaml:"shmemPmdMapped"`
	CmaTotal          uint64 `yaml:"cmaTotal"`
	CmaFree           uint64 `yaml:"cmaFree"`
	HugePagesTotal    uint64 `yaml:"hugePagesTotal"`
	HugePagesFree     uint64 `yaml:"hugePagesFree"`
	HugePagesRsvd     uint64 `yaml:"hugePagesRsvd"`
	HugePagesSurp     uint64 `yaml:"hugePagesSurp"`
	Hugepagesize      uint64 `yaml:"hugepagesize"`
	DirectMap4k       uint64 `yaml:"directMap4k"`
	DirectMap2m       uint64 `yaml:"directMap2m"`
	DirectMap1g       uint64 `yaml:"directMap1g"`
}

// NewMemory creates new default Memory stats object.
func NewMemory() *Memory {
	r := &Memory{
		md: resource.NewMetadata(NamespaceName, MemoryType, MemoryID, resource.VersionUndefined),
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Memory) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Memory) Spec() interface{} {
	return &r.spec
}

func (r *Memory) String() string {
	return fmt.Sprintf("secrets.MemorySecrets(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Memory) DeepCopy() resource.Resource {
	return &Memory{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Memory) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MemoryType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Used",
				JSONPath: "{.used}",
			},
			{
				Name:     "Total",
				JSONPath: "{.total}",
			},
		},
	}
}

// Update current Mem snapshot.
func (r *Memory) Update(info *procfs.Meminfo) {
	r.spec = MemorySpec{
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
