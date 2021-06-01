// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/prometheus/procfs"
)

// CPUType is type of Etcd resource.
const CPUType = resource.Type("CPUStats.perf.talos.dev")

// CPUID is a resource ID of singleton instance.
const CPUID = resource.ID("latest")

// CPU represents the last CPU stats snapshot.
type CPU struct {
	md   resource.Metadata
	spec CPUSpec
}

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
	r := &CPU{
		md: resource.NewMetadata(NamespaceName, CPUType, CPUID, resource.VersionUndefined),
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *CPU) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *CPU) Spec() interface{} {
	return &r.spec
}

func (r *CPU) String() string {
	return fmt.Sprintf("secrets.CPUSecrets(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *CPU) DeepCopy() resource.Resource {
	return &CPU{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *CPU) ResourceDefinition() meta.ResourceDefinitionSpec {
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

// Update current CPU snapshot.
func (r *CPU) Update(stat *procfs.Stat) {
	translateCPUStat := func(in procfs.CPUStat) CPUStat {
		return CPUStat{
			User:      in.User,
			Nice:      in.Nice,
			System:    in.System,
			Idle:      in.Idle,
			Iowait:    in.Iowait,
			Irq:       in.IRQ,
			SoftIrq:   in.SoftIRQ,
			Steal:     in.Steal,
			Guest:     in.Guest,
			GuestNice: in.GuestNice,
		}
	}

	translateListOfCPUStat := func(in []procfs.CPUStat) []CPUStat {
		res := make([]CPUStat, len(in))

		for i := range in {
			res[i] = translateCPUStat(in[i])
		}

		return res
	}

	r.spec = CPUSpec{
		CPUTotal:        translateCPUStat(stat.CPUTotal),
		CPU:             translateListOfCPUStat(stat.CPU),
		IRQTotal:        stat.IRQTotal,
		ContextSwitches: stat.ContextSwitches,
		ProcessCreated:  stat.ProcessCreated,
		ProcessRunning:  stat.ProcessesRunning,
		ProcessBlocked:  stat.ProcessesBlocked,
		SoftIrqTotal:    stat.SoftIRQTotal,
	}
}
