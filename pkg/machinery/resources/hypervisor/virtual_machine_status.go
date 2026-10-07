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

// VirtualMachineStatusType is the type of the VirtualMachineStatus resource.
const VirtualMachineStatusType = resource.Type("VirtualMachineStatuses.hypervisor.talos.dev")

// VirtualMachineStatus reports the last observed state of a configured virtual machine.
type VirtualMachineStatus = typed.Resource[VirtualMachineStatusSpec, VirtualMachineStatusExtension]

// VirtualMachineStatusSpec describes observed power state and reconciliation stage.
//
//gotagsrewrite:gen
type VirtualMachineStatusSpec struct {
	// PowerState is the matched domain's observed power, or unknown when no domain was observed.
	PowerState VirtualMachinePowerState `yaml:"powerState" protobuf:"1"`
	// Stage distinguishes unknown observation, convergence, readiness, and observed obstacles.
	Stage VirtualMachineStage `yaml:"stage" protobuf:"2"`
	// Error describes an observation failure or obstacle, when known.
	Error string `yaml:"error,omitempty" protobuf:"3"`
	// Addresses is the flat list of guest IP addresses in CIDR form, as reported by qemu-guest-agent.
	Addresses []string `yaml:"addresses,omitempty" protobuf:"4"`
	// Interfaces is the per-interface view returned by qemu-guest-agent.
	Interfaces []VirtualMachineGuestInterfaceSpec `yaml:"interfaces,omitempty" protobuf:"5"`
}

// NewVirtualMachineStatus initializes a VirtualMachineStatus resource.
func NewVirtualMachineStatus(namespace resource.Namespace, id resource.ID) *VirtualMachineStatus {
	return typed.NewResource[VirtualMachineStatusSpec, VirtualMachineStatusExtension](
		resource.NewMetadata(namespace, VirtualMachineStatusType, id, resource.VersionUndefined),
		VirtualMachineStatusSpec{},
	)
}

// VirtualMachineStatusExtension is auxiliary resource data for VirtualMachineStatus.
type VirtualMachineStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (VirtualMachineStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Power",
				JSONPath: "{.powerState}",
			},
			{
				Name:     "Stage",
				JSONPath: "{.stage}",
			},
			{
				Name:     "Addresses",
				JSONPath: "{.addresses}",
			},
			{
				Name:     "Error",
				JSONPath: "{.error}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineStatusType, &VirtualMachineStatus{}); err != nil {
		panic(err)
	}
}
