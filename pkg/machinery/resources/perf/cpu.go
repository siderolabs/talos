// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

//nolint:lll
//go:generate deep-copy -type CPUSpec -type MemorySpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// CPUType is type of Etcd resource.
const CPUType = resource.Type("CPUStats.perf.talos.dev")

// CPUID is a resource ID of singleton instance.
const CPUID = resource.ID("latest")

// CPU represents the last CPU stats snapshot.
type CPU = typed.Resource[CPUSpec, CPURD]

// CPUSpec represents the last CPU stats snapshot.
type CPUSpec struct {
	CPU             []CPUStat `yaml:"cpu"`
	CPUTotal        CPUStat   `yaml:"cpuTotal"`
	IRQTotal        uint64    `yaml:"irqTotal"`
	ContextSwitches uint64    `yaml:"contextSwitches"`
	ProcessCreated  uint64    `yaml:"processCreated"`
	ProcessRunning  uint64    `yaml:"processRunning"`
	ProcessBlocked  uint64    `yaml:"processBlocked"`
	SoftIrqTotal    uint64    `yaml:"softIrqTotal"`
}

// CPUStat represents a single cpu stat.
type CPUStat struct {
	User      float64 `yaml:"user"`
	Nice      float64 `yaml:"nice"`
	System    float64 `yaml:"system"`
	Idle      float64 `yaml:"idle"`
	Iowait    float64 `yaml:"iowait"`
	Irq       float64 `yaml:"irq"`
	SoftIrq   float64 `yaml:"softIrq"`
	Steal     float64 `yaml:"steal"`
	Guest     float64 `yaml:"guest"`
	GuestNice float64 `yaml:"guestNice"`
}

// NewCPU creates new default CPU stats object.
func NewCPU() *CPU {
	return typed.NewResource[CPUSpec, CPURD](
		resource.NewMetadata(NamespaceName, CPUType, CPUID, resource.VersionUndefined),
		CPUSpec{},
	)
}

// CPURD is an auxiliary type for CPU resource.
type CPURD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (CPURD) ResourceDefinition(resource.Metadata, CPUSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CPUType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "User",
				JSONPath: "{.cpuTotal.user}",
			},
			{
				Name:     "System",
				JSONPath: "{.cpuTotal.system}",
			},
		},
	}
}
