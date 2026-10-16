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

// VirtualMachineStopModeType is the type of the VirtualMachineStopMode resource.
const VirtualMachineStopModeType = resource.Type("VirtualMachineStopModes.hypervisor.talos.dev")

// VirtualMachineStopModeOwner is the owner every VirtualMachineStopMode is written under. It is the
// name of the controller which retires the records, which is what allows that controller to destroy
// records the hypervisor API writes.
const VirtualMachineStopModeOwner = "hypervisor.VirtualMachineStopModeController"

// VirtualMachineStopMode records how the next stop of a virtual machine is to be carried out:
// gracefully, by pressing the virtual ACPI power button, or forced, by destroying its domain.
//
// The ID is the virtual machine name. This is a parameter of a transition rather than a desired
// state, which is why it does not live in the machine configuration: once a virtual machine is
// stopped there is nothing left for it to say, and a domain is transient, so a stop can never be
// in flight across a reboot of the host. It is written by the hypervisor API and destroyed by
// VirtualMachineStopModeController once the stop it describes is over. A virtual machine with none,
// such as one stopped by a machine configuration patch, is stopped as if forced: its domain is
// destroyed, without asking the guest.
type VirtualMachineStopMode = typed.Resource[VirtualMachineStopModeSpec, VirtualMachineStopModeExtension]

// VirtualMachineStopModeSpec is the spec for VirtualMachineStopMode.
//
//gotagsrewrite:gen
type VirtualMachineStopModeSpec struct {
	// Mode is how the next stop is to be carried out.
	Mode string `yaml:"mode" protobuf:"1"`
}

// NewVirtualMachineStopMode initializes a VirtualMachineStopMode resource.
func NewVirtualMachineStopMode(namespace resource.Namespace, id resource.ID) *VirtualMachineStopMode {
	return typed.NewResource[VirtualMachineStopModeSpec, VirtualMachineStopModeExtension](
		resource.NewMetadata(namespace, VirtualMachineStopModeType, id, resource.VersionUndefined),
		VirtualMachineStopModeSpec{},
	)
}

// VirtualMachineStopModeExtension is auxiliary resource data for VirtualMachineStopMode.
type VirtualMachineStopModeExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VirtualMachineStopModeExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineStopModeType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Mode",
				JSONPath: "{.mode}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineStopModeType, &VirtualMachineStopMode{}); err != nil {
		panic(err)
	}
}
