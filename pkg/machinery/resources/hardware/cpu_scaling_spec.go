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

// CPUScalingSpecType is type of CPUScalingSpec resource.
const CPUScalingSpecType = resource.Type("CPUScalingSpecs.hardware.talos.dev")

// CPUScalingSpec resource holds the settings to apply to a single Linux cpufreq policy.
//
// The resource ID is the cpufreq policy name (`policy0`, `policy1`, ...).
type CPUScalingSpec = typed.Resource[CPUScalingSpecSpec, CPUScalingSpecExtension]

// CPUScalingSpecSpec describes the cpufreq settings requested for a policy.
//
// An empty field leaves that attribute alone.
//
//gotagsrewrite:gen
type CPUScalingSpecSpec struct {
	// Governor is the scaling governor to set.
	Governor string `yaml:"governor,omitempty" protobuf:"1"`
	// EnergyPerformancePreference is the energy performance preference to set.
	EnergyPerformancePreference string `yaml:"energyPerformancePreference,omitempty" protobuf:"2"`
	// MinFrequencyKhz is the lower bound of the frequency window to set, in kHz.
	MinFrequencyKhz uint64 `yaml:"minFrequencyKhz,omitempty" protobuf:"3"`
	// MaxFrequencyKhz is the upper bound of the frequency window to set, in kHz.
	MaxFrequencyKhz uint64 `yaml:"maxFrequencyKhz,omitempty" protobuf:"4"`

	// ConfigName is the CPUScalingConfig document these settings came from.
	ConfigName string `yaml:"configName,omitempty" protobuf:"5"`
}

// NewCPUScalingSpec initializes a CPUScalingSpec resource.
func NewCPUScalingSpec(id string) *CPUScalingSpec {
	return typed.NewResource[CPUScalingSpecSpec, CPUScalingSpecExtension](
		resource.NewMetadata(NamespaceName, CPUScalingSpecType, id, resource.VersionUndefined),
		CPUScalingSpecSpec{},
	)
}

// CPUScalingSpecExtension provides auxiliary methods for CPUScalingSpec info.
type CPUScalingSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (CPUScalingSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: CPUScalingSpecType,
		Aliases: []resource.Type{
			"cpuscalingspec",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Governor",
				JSONPath: `{.governor}`,
			},
			{
				Name:     "EPP",
				JSONPath: `{.energyPerformancePreference}`,
			},
			{
				Name:     "Min kHz",
				JSONPath: `{.minFrequencyKhz}`,
			},
			{
				Name:     "Max kHz",
				JSONPath: `{.maxFrequencyKhz}`,
			},
			{
				Name:     "Config",
				JSONPath: `{.configName}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[CPUScalingSpecSpec](CPUScalingSpecType, &CPUScalingSpec{})
	if err != nil {
		panic(err)
	}
}
