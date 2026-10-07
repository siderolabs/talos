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

// WorkloadMemorySpecType is type of WorkloadMemorySpec resource.
const WorkloadMemorySpecType = resource.Type("WorkloadMemorySpecs.runtime.talos.dev")

// WorkloadMemorySpec is the canonical desired memory ceiling for the Talos-owned workload cgroup roots.
//
// A missing resource means projection is pending, not that enforcement is disabled.
type WorkloadMemorySpec = typed.Resource[WorkloadMemorySpecSpec, WorkloadMemorySpecExtension]

// WorkloadMemorySpecID is the ID of the singleton WorkloadMemorySpec.
const WorkloadMemorySpecID resource.ID = "workload-memory"

// WorkloadMemorySpecSpec describes the desired memory.max for each Talos-owned workload root, in bytes.
//
// Zero means no limit is configured for that root; roots are independent.
//
//gotagsrewrite:gen
type WorkloadMemorySpecSpec struct {
	TalosContainersLimit uint64 `yaml:"talosContainersLimit,omitempty" protobuf:"1"`
	VirtualMachinesLimit uint64 `yaml:"virtualMachinesLimit,omitempty" protobuf:"2"`
}

// NewWorkloadMemorySpec initializes a WorkloadMemorySpec resource.
func NewWorkloadMemorySpec() *WorkloadMemorySpec {
	return typed.NewResource[WorkloadMemorySpecSpec, WorkloadMemorySpecExtension](
		resource.NewMetadata(NamespaceName, WorkloadMemorySpecType, WorkloadMemorySpecID, resource.VersionUndefined),
		WorkloadMemorySpecSpec{},
	)
}

// WorkloadMemorySpecExtension is auxiliary resource data for WorkloadMemorySpec.
type WorkloadMemorySpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (WorkloadMemorySpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             WorkloadMemorySpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Talos Containers",
				JSONPath: `{.talosContainersLimit}`,
			},
			{
				Name:     "Virtual Machines",
				JSONPath: `{.virtualMachinesLimit}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[WorkloadMemorySpecSpec](WorkloadMemorySpecType, &WorkloadMemorySpec{})
	if err != nil {
		panic(err)
	}
}
