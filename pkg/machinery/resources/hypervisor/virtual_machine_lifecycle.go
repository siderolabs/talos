// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VirtualMachineLifecycleType is type of VirtualMachineLifecycle resource.
const VirtualMachineLifecycleType = resource.Type("VirtualMachineLifecycles.hypervisor.talos.dev")

// VirtualMachineLifecycleID is the singleton ID of the resource.
const VirtualMachineLifecycleID = resource.ID("virtual-machines")

// VirtualMachineLifecycle resource exists so that virtual machines can be stopped gracefully on the way down.
type VirtualMachineLifecycle = typed.Resource[VirtualMachineLifecycleSpec, VirtualMachineLifecycleExtension]

// VirtualMachineLifecycleSpec is the spec for VirtualMachineLifecycle.
//
//gotagsrewrite:gen
type VirtualMachineLifecycleSpec struct {
	// ShutdownDeadline is set when the resource is torn down: all virtual machines have to be stopped by then.
	ShutdownDeadline time.Time `yaml:"shutdownDeadline,omitempty" protobuf:"1"`
}

// NewVirtualMachineLifecycle initializes a VirtualMachineLifecycle resource.
func NewVirtualMachineLifecycle(namespace resource.Namespace, id resource.ID) *VirtualMachineLifecycle {
	return typed.NewResource[VirtualMachineLifecycleSpec, VirtualMachineLifecycleExtension](
		resource.NewMetadata(namespace, VirtualMachineLifecycleType, id, resource.VersionUndefined),
		VirtualMachineLifecycleSpec{},
	)
}

// VirtualMachineLifecycleExtension is auxiliary resource data for VirtualMachineLifecycle.
type VirtualMachineLifecycleExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VirtualMachineLifecycleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineLifecycleType,
		Aliases:          []resource.Type{"vmlifecycle", "virtualmachinelifecycle", "virtualmachinelifecycles"},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineLifecycleType, &VirtualMachineLifecycle{}); err != nil {
		panic(err)
	}
}
