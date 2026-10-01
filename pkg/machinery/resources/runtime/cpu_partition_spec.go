// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// CPUPartitionSpecType is type of CPUPartitionSpec resource.
const CPUPartitionSpecType = resource.Type("CPUPartitionSpecs.runtime.talos.dev")

// CPUPartitionSpec is the canonical desired CPU partition policy of the machine.
//
// A missing resource means projection is pending, not that enforcement is disabled.
type CPUPartitionSpec = typed.Resource[CPUPartitionSpecSpec, CPUPartitionSpecExtension]

// CPUPartitionSpecID is the ID of the singleton CPUPartitionSpec.
const CPUPartitionSpecID resource.ID = "cpu-partition"

// CPUPartitionSpecSpec describes the desired CPU partition policy.
//
//gotagsrewrite:gen
type CPUPartitionSpecSpec struct {
	Enabled bool `yaml:"enabled" protobuf:"1"`
	// Omitted roots are unrestricted.
	Roots  map[string]string       `yaml:"roots,omitempty" protobuf:"2"`
	Slices []CPUPartitionSliceSpec `yaml:"slices,omitempty" protobuf:"3"`
}

// CPUPartitionSliceSpec is one named subset of the virtual machine root.
//
//gotagsrewrite:gen
type CPUPartitionSliceSpec struct {
	Name      string `yaml:"name" protobuf:"1"`
	CPUs      string `yaml:"cpus" protobuf:"2"`
	Exclusive bool   `yaml:"exclusive" protobuf:"3"`
}

// NewCPUPartitionSpec initializes a CPUPartitionSpec resource.
func NewCPUPartitionSpec() *CPUPartitionSpec {
	return typed.NewResource[CPUPartitionSpecSpec, CPUPartitionSpecExtension](
		resource.NewMetadata(NamespaceName, CPUPartitionSpecType, CPUPartitionSpecID, resource.VersionUndefined),
		CPUPartitionSpecSpec{},
	)
}

// CPUPartitionSpecExtension is auxiliary resource data for CPUPartitionSpec.
type CPUPartitionSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (CPUPartitionSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CPUPartitionSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Enabled",
				JSONPath: `{.enabled}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[CPUPartitionSpecSpec](CPUPartitionSpecType, &CPUPartitionSpec{})
	if err != nil {
		panic(err)
	}
}
