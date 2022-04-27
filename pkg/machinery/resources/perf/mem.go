// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// MemoryType is type of Etcd resource.
const MemoryType = resource.Type("MemoryStats.perf.talos.dev")

// MemoryID is a resource ID of singleton instance.
const MemoryID = resource.ID("latest")

// Memory represents the last Memory stats snapshot.
type Memory = typed.Resource[MemorySpec, MemoryRD]

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
	return typed.NewResource[MemorySpec, MemoryRD](
		resource.NewMetadata(NamespaceName, MemoryType, MemoryID, resource.VersionUndefined),
		MemorySpec{},
	)
}

// DeepCopy implements typed.Deepcopyable interface.
func (spec MemorySpec) DeepCopy() MemorySpec {
	return spec
}

// MemoryRD is an auxiliary type for Memory resource.
type MemoryRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MemoryRD) ResourceDefinition(resource.Metadata, MemorySpec) meta.ResourceDefinitionSpec {
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
