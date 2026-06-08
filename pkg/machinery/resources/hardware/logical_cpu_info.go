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

// LogicalCPUInfoType is type of LogicalCPUInfo resource.
//
// "LogicalCPUInfo" rather than "LogicalCPU" because the COSI ResourceDefinition
// validator runs the name through go-pluralize, which does not recognize "CPUs"
// (the trailing acronym throws it off). The "Infos" suffix gives pluralize a
// plural form it can validate.
const LogicalCPUInfoType = resource.Type("LogicalCPUInfos.hardware.talos.dev")

// LogicalCPUInfo holds per-logical-CPU information sourced from the running kernel.
//
// Unlike Processor (which is SMBIOS-derived and socket-level), LogicalCPUInfo
// entries are per-logical-CPU and report runtime state that the kernel may
// apply or update per core.
type LogicalCPUInfo = typed.Resource[LogicalCPUInfoSpec, LogicalCPUInfoExtension]

// LogicalCPUInfoSpec represents a single logical CPU.
//
//gotagsrewrite:gen
type LogicalCPUInfoSpec struct {
	// Microcode revision (x86-only; empty elsewhere).
	Microcode string `yaml:"microcode,omitempty" protobuf:"1"`
	// Socket from /sys/devices/system/cpu/cpuN/topology/physical_package_id.
	Socket uint32 `yaml:"socket" protobuf:"2"`
	// Core from /sys/devices/system/cpu/cpuN/topology/core_id. SMT threads on
	// the same physical core share this value.
	Core uint32 `yaml:"core" protobuf:"3"`
	// NumaNode resolved from /sys/devices/system/cpu/cpuN/node<N>. Distinct
	// from Socket on sub-NUMA-clustered systems (e.g. AMD NPS2/NPS4, Intel SNC).
	NumaNode uint32 `yaml:"numaNode" protobuf:"5"`
	// Bugs lists hardware vulnerabilities reported by the kernel (x86-only).
	Bugs []string `yaml:"bugs,omitempty" protobuf:"4"`
}

// NewLogicalCPUInfo initializes a LogicalCPUInfo resource.
func NewLogicalCPUInfo(id string) *LogicalCPUInfo {
	return typed.NewResource[LogicalCPUInfoSpec, LogicalCPUInfoExtension](
		resource.NewMetadata(NamespaceName, LogicalCPUInfoType, id, resource.VersionUndefined),
		LogicalCPUInfoSpec{},
	)
}

// LogicalCPUInfoExtension provides auxiliary methods for LogicalCPUInfo.
type LogicalCPUInfoExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LogicalCPUInfoExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: LogicalCPUInfoType,
		Aliases: []resource.Type{
			"logicalcpus",
			"logicalcpu",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Microcode",
				JSONPath: `{.microcode}`,
			},
			{
				Name:     "Socket",
				JSONPath: `{.socket}`,
			},
			{
				Name:     "Core",
				JSONPath: `{.core}`,
			},
			{
				Name:     "NUMA",
				JSONPath: `{.numaNode}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LogicalCPUInfoSpec](LogicalCPUInfoType, &LogicalCPUInfo{})
	if err != nil {
		panic(err)
	}
}
