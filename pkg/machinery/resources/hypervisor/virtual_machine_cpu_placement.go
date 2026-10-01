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

// VirtualMachineCPUPlacement is the cgroup partition the CPU partition coordinator admits a
// virtual machine into.
//
// The ID is the virtual machine name. The placement is a grant: the runtime starts the domain
// only in this partition, and claims the placement (finalizer) for as long as the domain may
// be running. The coordinator never changes a claimed placement's partition; it tears the
// placement down and waits for the claim to be released (domain actually removed) before
// destroying it. A placement's CPUs are not recorded here: they are the partition's applied
// mask in CPUPartitionStatus and may change live without touching the placement.
type VirtualMachineCPUPlacement = typed.Resource[VirtualMachineCPUPlacementSpec, VirtualMachineCPUPlacementExtension]

// VirtualMachineCPUPlacementSpec is the spec for VirtualMachineCPUPlacement.
//
//gotagsrewrite:gen
type VirtualMachineCPUPlacementSpec struct {
	// Partition is the libvirt resource partition path, e.g. /virtualmachines.partition/shared.partition.
	Partition string `yaml:"partition" protobuf:"1"`
	// Slice is the CPU partition slice name; empty for the shared remainder.
	Slice string `yaml:"slice,omitempty" protobuf:"2"`
	// Exclusive is true when the slice is exclusive to this virtual machine.
	Exclusive bool `yaml:"exclusive" protobuf:"3"`
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
