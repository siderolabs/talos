// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MemoryType is type of Etcd resource.
const MemoryType = resource.Type("MemoryStats.perf.talos.dev")

// MemoryID is a resource ID of singleton instance.
const MemoryID = resource.ID("latest")

// Memory represents the last Memory stats snapshot.
type Memory = typed.Resource[MemorySpec, MemoryExtension]

// MemorySpec represents the last Memory stats snapshot.
//
//gotagsrewrite:gen
type MemorySpec struct {
	MemTotal          uint64 `yaml:"total" protobuf:"1"`
	MemUsed           uint64 `yaml:"used" protobuf:"2"`
	MemAvailable      uint64 `yaml:"available" protobuf:"3"`
	Buffers           uint64 `yaml:"buffers" protobuf:"4"`
	Cached            uint64 `yaml:"cached" protobuf:"5"`
	SwapCached        uint64 `yaml:"swapCached" protobuf:"6"`
	Active            uint64 `yaml:"active" protobuf:"7"`
	Inactive          uint64 `yaml:"inactive" protobuf:"8"`
	ActiveAnon        uint64 `yaml:"activeAnon" protobuf:"9"`
	InactiveAnon      uint64 `yaml:"inactiveAnon" protobuf:"10"`
	ActiveFile        uint64 `yaml:"activeFile" protobuf:"11"`
	InactiveFile      uint64 `yaml:"inactiveFile" protobuf:"12"`
	Unevictable       uint64 `yaml:"unevictable" protobuf:"13"`
	Mlocked           uint64 `yaml:"mlocked" protobuf:"14"`
	SwapTotal         uint64 `yaml:"swapTotal" protobuf:"15"`
	SwapFree          uint64 `yaml:"swapFree" protobuf:"16"`
	Dirty             uint64 `yaml:"dirty" protobuf:"17"`
	Writeback         uint64 `yaml:"writeback" protobuf:"18"`
	AnonPages         uint64 `yaml:"anonPages" protobuf:"19"`
	Mapped            uint64 `yaml:"mapped" protobuf:"20"`
	Shmem             uint64 `yaml:"shmem" protobuf:"21"`
	Slab              uint64 `yaml:"slab" protobuf:"22"`
	SReclaimable      uint64 `yaml:"sreclaimable" protobuf:"23"`
	SUnreclaim        uint64 `yaml:"sunreclaim" protobuf:"24"`
	KernelStack       uint64 `yaml:"kernelStack" protobuf:"25"`
	PageTables        uint64 `yaml:"pageTables" protobuf:"26"`
	NFSunstable       uint64 `yaml:"nfsunstable" protobuf:"27"`
	Bounce            uint64 `yaml:"bounce" protobuf:"28"`
	WritebackTmp      uint64 `yaml:"writeBacktmp" protobuf:"29"`
	CommitLimit       uint64 `yaml:"commitLimit" protobuf:"30"`
	CommittedAS       uint64 `yaml:"commitTedas" protobuf:"31"`
	VmallocTotal      uint64 `yaml:"vmallocTotal" protobuf:"32"`
	VmallocUsed       uint64 `yaml:"vmallocUsed" protobuf:"33"`
	VmallocChunk      uint64 `yaml:"vmallocChunk" protobuf:"34"`
	HardwareCorrupted uint64 `yaml:"hardwareCorrupted" protobuf:"35"`
	AnonHugePages     uint64 `yaml:"anonHugePages" protobuf:"36"`
	ShmemHugePages    uint64 `yaml:"shmemHugePages" protobuf:"37"`
	ShmemPmdMapped    uint64 `yaml:"shmemPmdMapped" protobuf:"38"`
	CmaTotal          uint64 `yaml:"cmaTotal" protobuf:"39"`
	CmaFree           uint64 `yaml:"cmaFree" protobuf:"40"`
	HugePagesTotal    uint64 `yaml:"hugePagesTotal" protobuf:"41"`
	HugePagesFree     uint64 `yaml:"hugePagesFree" protobuf:"42"`
	HugePagesRsvd     uint64 `yaml:"hugePagesRsvd" protobuf:"43"`
	HugePagesSurp     uint64 `yaml:"hugePagesSurp" protobuf:"44"`
	Hugepagesize      uint64 `yaml:"hugepagesize" protobuf:"45"`
	DirectMap4k       uint64 `yaml:"directMap4k" protobuf:"46"`
	DirectMap2m       uint64 `yaml:"directMap2m" protobuf:"47"`
	DirectMap1g       uint64 `yaml:"directMap1g" protobuf:"48"`
}

// NewMemory creates new default Memory stats object.
func NewMemory() *Memory {
	return typed.NewResource[MemorySpec, MemoryExtension](
		resource.NewMetadata(NamespaceName, MemoryType, MemoryID, resource.VersionUndefined),
		MemorySpec{},
	)
}

// MemoryExtension is an auxiliary type for Memory resource.
type MemoryExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MemoryExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
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

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MemorySpec](MemoryType, &Memory{})
	if err != nil {
		panic(err)
	}
}
