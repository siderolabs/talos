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

// VirtualMachinePowerOverrideType is the type of the VirtualMachinePowerOverride resource.
const VirtualMachinePowerOverrideType = resource.Type("VirtualMachinePowerOverrides.hypervisor.talos.dev")

// VirtualMachinePowerOverride overrides the configured power state of a virtual machine for the
// current boot.
//
// It is written by the hypervisor API and never persisted: a reboot drops it, and the virtual
// machine goes back to the power state its configuration declares. While it exists it wins over
// that declaration, whatever the configuration is changed to.
type VirtualMachinePowerOverride = typed.Resource[VirtualMachinePowerOverrideSpec, VirtualMachinePowerOverrideExtension]

// VirtualMachinePowerOverrideSpec describes the power state a virtual machine is driven towards.
//
//gotagsrewrite:gen
type VirtualMachinePowerOverrideSpec struct {
	// PowerState overrides the configured power state.
	PowerState string `yaml:"powerState" protobuf:"1"`
	// StopRequest is the stop asked for, encoded as "<mode>/<id>"; empty unless stopped.
	StopRequest string `yaml:"stopRequest,omitempty" protobuf:"2"`
}

// NewVirtualMachinePowerOverride initializes a VirtualMachinePowerOverride resource.
func NewVirtualMachinePowerOverride(namespace resource.Namespace, id resource.ID) *VirtualMachinePowerOverride {
	return typed.NewResource[VirtualMachinePowerOverrideSpec, VirtualMachinePowerOverrideExtension](
		resource.NewMetadata(namespace, VirtualMachinePowerOverrideType, id, resource.VersionUndefined),
		VirtualMachinePowerOverrideSpec{},
	)
}

// VirtualMachinePowerOverrideExtension is auxiliary resource data for VirtualMachinePowerOverride.
type VirtualMachinePowerOverrideExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (VirtualMachinePowerOverrideExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachinePowerOverrideType,
		Aliases:          []resource.Type{"vmpoweroverride"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Power",
				JSONPath: "{.powerState}",
			},
			{
				Name:     "Stop Request",
				JSONPath: "{.stopRequest}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachinePowerOverrideType, &VirtualMachinePowerOverride{}); err != nil {
		panic(err)
	}
}
