// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VirtualMachineProfileSpecType is the type of a guest hardware profile.
const VirtualMachineProfileSpecType = resource.Type("VirtualMachineProfileSpecs.hypervisor.talos.dev")

// VirtualMachineProfileSpec is a named guest hardware contract.
type VirtualMachineProfileSpec = typed.Resource[VirtualMachineProfileSpecSpec, VirtualMachineProfileSpecExtension]

// VirtualMachineProfileSpecSpec is the spec for VirtualMachineProfileSpec.
//
//gotagsrewrite:gen
type VirtualMachineProfileSpecSpec struct {
	Builtin bool                         `yaml:"builtin" protobuf:"1"`
	Machine string                       `yaml:"machine" protobuf:"2"`
	CPU     VirtualMachineProfileCPUSpec `yaml:"cpu" protobuf:"3"`
}

// VirtualMachineProfileCPUSpec defines a guest-visible CPU model.
//
//gotagsrewrite:gen
type VirtualMachineProfileCPUSpec struct {
	Mode     hypervisorhelpers.VirtualMachineProfileCPUMode `yaml:"mode" protobuf:"1"`
	Model    string                                         `yaml:"model,omitempty" protobuf:"2"`
	Fallback string                                         `yaml:"fallback,omitempty" protobuf:"3"`
	Features []VirtualMachineProfileCPUFeatureSpec          `yaml:"features,omitempty" protobuf:"4"`
}

// VirtualMachineProfileCPUFeatureSpec defines a guest CPU feature policy.
//
//gotagsrewrite:gen
type VirtualMachineProfileCPUFeatureSpec struct {
	Name   string                                                  `yaml:"name" protobuf:"1"`
	Policy hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicy `yaml:"policy" protobuf:"2"`
}

// NewVirtualMachineProfileSpec constructs a named profile resource.
func NewVirtualMachineProfileSpec(namespace resource.Namespace, id resource.ID) *VirtualMachineProfileSpec {
	return typed.NewResource[VirtualMachineProfileSpecSpec, VirtualMachineProfileSpecExtension](
		resource.NewMetadata(namespace, VirtualMachineProfileSpecType, id, resource.VersionUndefined),
		VirtualMachineProfileSpecSpec{},
	)
}

// VirtualMachineProfileSpecExtension supplies resource registration.
type VirtualMachineProfileSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (VirtualMachineProfileSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineProfileSpecType,
		Aliases:          []resource.Type{"vmprofile", "vmprofiles"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Builtin", JSONPath: `{.builtin}`},
			{Name: "Machine", JSONPath: `{.machine}`},
			{Name: "CPU Mode", JSONPath: `{.cpu.mode}`},
			{Name: "Model", JSONPath: `{.cpu.model}`},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineProfileSpecType, &VirtualMachineProfileSpec{}); err != nil {
		panic(err)
	}
}
