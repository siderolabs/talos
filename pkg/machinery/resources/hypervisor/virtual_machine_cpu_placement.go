// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VirtualMachineCPUPlacementType is the type of the VirtualMachineCPUPlacement resource.
const VirtualMachineCPUPlacementType = resource.Type("VirtualMachineCPUPlacements.hypervisor.talos.dev")

// VirtualMachineCPUPlacement grants a VM access to a cgroup partition.
//
// The runtime holds a finalizer until the domain is removed. A claimed partition
// must not change, though its CPU mask may be updated independently.
type VirtualMachineCPUPlacement = typed.Resource[VirtualMachineCPUPlacementSpec, VirtualMachineCPUPlacementExtension]

// VirtualMachineCPUPlacementSpec is the spec for VirtualMachineCPUPlacement.
//
//gotagsrewrite:gen
type VirtualMachineCPUPlacementSpec struct {
	Partition string `yaml:"partition" protobuf:"1"`
	Slice     string `yaml:"slice,omitempty" protobuf:"2"`
	Exclusive bool   `yaml:"exclusive" protobuf:"3"`
}

// NewVirtualMachineCPUPlacement initializes a VirtualMachineCPUPlacement resource.
func NewVirtualMachineCPUPlacement(namespace resource.Namespace, id resource.ID) *VirtualMachineCPUPlacement {
	return typed.NewResource[VirtualMachineCPUPlacementSpec, VirtualMachineCPUPlacementExtension](
		resource.NewMetadata(namespace, VirtualMachineCPUPlacementType, id, resource.VersionUndefined),
		VirtualMachineCPUPlacementSpec{},
	)
}

// VirtualMachineCPUPlacementExtension is auxiliary resource data for VirtualMachineCPUPlacement.
type VirtualMachineCPUPlacementExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VirtualMachineCPUPlacementExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineCPUPlacementType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Partition",
				JSONPath: "{.partition}",
			},
			{
				Name:     "Exclusive",
				JSONPath: "{.exclusive}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineCPUPlacementType, &VirtualMachineCPUPlacement{}); err != nil {
		panic(err)
	}
}
