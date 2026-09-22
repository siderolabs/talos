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

// CPUScalingStatusType is type of CPUScalingStatus resource.
const CPUScalingStatusType = resource.Type("CPUScalingStatuses.hardware.talos.dev")

// CPUScalingStatus resource holds the state of a single Linux cpufreq policy.
//
// The resource ID is the cpufreq policy name (`policy0`, `policy1`, ...). A policy, not a logical
// CPU, is the unit of scaling control: `/sys/devices/system/cpu/cpuN/cpufreq` is a symlink into
// `/sys/devices/system/cpu/cpufreq/policyM`, and one policy may drive several CPUs.
type CPUScalingStatus = typed.Resource[CPUScalingStatusSpec, CPUScalingStatusExtension]

// CPUScalingStatusSpec describes a cpufreq policy: which CPUs it drives, what its driver supports,
// and the governor, energy performance preference and frequency limits currently in effect.
//
// Only values which are stable unless something sets them are reported. The current and requested
// frequencies are left out: the kernel moves them continuously, and reporting them would rewrite
// this resource on every read.
//
//gotagsrewrite:gen
type CPUScalingStatusSpec struct {
	// Driver is the cpufreq scaling driver backing the policy (e.g. `intel_pstate`, `intel_cpufreq`, `acpi-cpufreq`).
	Driver string `yaml:"driver,omitempty" protobuf:"1"`
	// AffectedCPUs is the sorted list of online logical CPUs whose frequency this policy controls.
	AffectedCPUs []uint32 `yaml:"affectedCPUs,omitempty" protobuf:"2"`
	// RelatedCPUs is the sorted list of all logical CPUs belonging to this policy, online or not.
	RelatedCPUs []uint32 `yaml:"relatedCPUs,omitempty" protobuf:"3"`

	// AvailableGovernors is the list of scaling governors the driver offers for this policy.
	AvailableGovernors []string `yaml:"availableGovernors,omitempty" protobuf:"4"`
	// AvailableEPPs is the list of energy performance preferences the driver offers, empty unless
	// the driver implements them (HWP-enabled intel_pstate, amd-pstate in active mode).
	AvailableEPPs []string `yaml:"availableEPPs,omitempty" protobuf:"5"`

	// Governor is the scaling governor currently in effect.
	Governor string `yaml:"governor,omitempty" protobuf:"6"`
	// EnergyPerformancePreference is the energy performance preference currently in effect.
	EnergyPerformancePreference string `yaml:"energyPerformancePreference,omitempty" protobuf:"7"`

	// CPUInfoMinFrequencyKhz is the lowest frequency the hardware supports, in kHz.
	CPUInfoMinFrequencyKhz uint64 `yaml:"cpuinfoMinFrequencyKhz,omitempty" protobuf:"8"`
	// CPUInfoMaxFrequencyKhz is the highest frequency the hardware supports, in kHz.
	CPUInfoMaxFrequencyKhz uint64 `yaml:"cpuinfoMaxFrequencyKhz,omitempty" protobuf:"9"`
	// BaseFrequencyKhz is the sustained (non-turbo) frequency, in kHz, reported only by some drivers.
	BaseFrequencyKhz uint64 `yaml:"baseFrequencyKhz,omitempty" protobuf:"10"`

	// CPUCapacity is the scheduler's capacity rating of the policy's first CPU, relative to 1024
	// for the most capable CPU in the system. On asymmetric systems it distinguishes big from little cores.
	CPUCapacity uint32 `yaml:"cpuCapacity,omitempty" protobuf:"11"`
	// CoreType is `performance` or `efficiency` on CPUs with a hybrid topology, empty otherwise.
	CoreType string `yaml:"coreType,omitempty" protobuf:"12"`

	// ScalingMinFrequencyKhz is the lowest frequency the policy currently allows the governor to pick, in kHz.
	ScalingMinFrequencyKhz uint64 `yaml:"scalingMinFrequencyKhz,omitempty" protobuf:"13"`
	// ScalingMaxFrequencyKhz is the highest frequency the policy currently allows the governor to pick, in kHz.
	ScalingMaxFrequencyKhz uint64 `yaml:"scalingMaxFrequencyKhz,omitempty" protobuf:"14"`
}

// CPU core types reported in CPUScalingStatusSpec.CoreType.
const (
	CoreTypePerformance = "performance"
	CoreTypeEfficiency  = "efficiency"
)

// NewCPUScalingStatus initializes a CPUScalingStatus resource.
func NewCPUScalingStatus(id string) *CPUScalingStatus {
	return typed.NewResource[CPUScalingStatusSpec, CPUScalingStatusExtension](
		resource.NewMetadata(NamespaceName, CPUScalingStatusType, id, resource.VersionUndefined),
		CPUScalingStatusSpec{},
	)
}

// CPUScalingStatusExtension provides auxiliary methods for CPUScalingStatus info.
type CPUScalingStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (CPUScalingStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: CPUScalingStatusType,
		Aliases: []resource.Type{
			"cpuscaling",
			"cpuscalingstatus",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Driver",
				JSONPath: `{.driver}`,
			},
			{
				Name:     "Governor",
				JSONPath: `{.governor}`,
			},
			{
				Name:     "CPUs",
				JSONPath: `{.affectedCPUs}`,
			},
			{
				Name:     "HW Min kHz",
				JSONPath: `{.cpuinfoMinFrequencyKhz}`,
			},
			{
				Name:     "HW Max kHz",
				JSONPath: `{.cpuinfoMaxFrequencyKhz}`,
			},
			{
				Name:     "Min kHz",
				JSONPath: `{.scalingMinFrequencyKhz}`,
			},
			{
				Name:     "Max kHz",
				JSONPath: `{.scalingMaxFrequencyKhz}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[CPUScalingStatusSpec](CPUScalingStatusType, &CPUScalingStatus{})
	if err != nil {
		panic(err)
	}
}
