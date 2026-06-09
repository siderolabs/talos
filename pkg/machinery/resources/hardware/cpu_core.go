// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// CPUCoreType is type of CPUCore resource.
const CPUCoreType = resource.Type("CPUCores.hardware.talos.dev")

// CPUCore resource holds the Linux kernel view of a single CPU core, as parsed from /proc/cpuinfo.
//
// A single resource is produced per physical core, aggregating all logical CPUs (hardware threads)
// which belong to that core.
type CPUCore = typed.Resource[CPUCoreSpec, CPUCoreExtension]

// CPUCoreSpec represents a single CPU core as seen by the Linux kernel.
//
//gotagsrewrite:gen
type CPUCoreSpec struct {
	// Socket is the physical package (socket) identifier the core belongs to.
	Socket string `yaml:"socket,omitempty" protobuf:"1"`
	// CoreID is the core identifier within the socket.
	CoreID string `yaml:"coreID,omitempty" protobuf:"2"`
	// LogicalCPUs is the sorted list of logical CPU (hardware thread) numbers belonging to this core.
	LogicalCPUs []uint32 `yaml:"logicalCPUs,omitempty" protobuf:"3"`
	// VendorID is the CPU vendor identifier (e.g. `GenuineIntel`, `AuthenticAMD`).
	VendorID string `yaml:"vendorID,omitempty" protobuf:"4"`
	// CPUFamily is the CPU family.
	CPUFamily string `yaml:"cpuFamily,omitempty" protobuf:"5"`
	// Model is the CPU model number.
	Model string `yaml:"model,omitempty" protobuf:"6"`
	// ModelName is the human-readable CPU model name.
	ModelName string `yaml:"modelName,omitempty" protobuf:"7"`
	// Stepping is the CPU stepping.
	Stepping string `yaml:"stepping,omitempty" protobuf:"8"`
	// Microcode is the microcode revision.
	Microcode string `yaml:"microcode,omitempty" protobuf:"9"`
	// CacheSize is the CPU cache size as reported by the kernel (e.g. `512 KB`).
	CacheSize string `yaml:"cacheSize,omitempty" protobuf:"10"`
	// CoresPerSocket is the number of cores in the socket this core belongs to.
	CoresPerSocket uint32 `yaml:"coresPerSocket,omitempty" protobuf:"11"`
	// ThreadsPerSocket is the number of logical CPUs (siblings) in the socket this core belongs to.
	ThreadsPerSocket uint32 `yaml:"threadsPerSocket,omitempty" protobuf:"12"`
	// Flags is the list of CPU feature flags.
	Flags []string `yaml:"flags,omitempty" protobuf:"13"`
	// Bugs is the list of known CPU bugs.
	Bugs []string `yaml:"bugs,omitempty" protobuf:"14"`
	// BogoMips is the kernel BogoMips measurement for the core.
	BogoMips float64 `yaml:"bogoMips,omitempty" protobuf:"15"`
	// AddressSizes describes the physical and virtual address sizes.
	AddressSizes string `yaml:"addressSizes,omitempty" protobuf:"16"`
}

// NewCPUCore initializes a CPUCore resource.
func NewCPUCore(id string) *CPUCore {
	return typed.NewResource[CPUCoreSpec, CPUCoreExtension](
		resource.NewMetadata(NamespaceName, CPUCoreType, id, resource.VersionUndefined),
		CPUCoreSpec{},
	)
}

// CPUCoreExtension provides auxiliary methods for CPUCore info.
type CPUCoreExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (CPUCoreExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: CPUCoreType,
		Aliases: []resource.Type{
			"cpucore",
			"cpucores",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Socket",
				JSONPath: `{.socket}`,
			},
			{
				Name:     "Core",
				JSONPath: `{.coreID}`,
			},
			{
				Name:     "Model",
				JSONPath: `{.modelName}`,
			},
			{
				Name:     "Logical CPUs",
				JSONPath: `{.logicalCPUs}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[CPUCoreSpec](CPUCoreType, &CPUCore{})
	if err != nil {
		panic(err)
	}
}
