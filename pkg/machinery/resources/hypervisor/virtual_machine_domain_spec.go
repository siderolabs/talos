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

// VirtualMachineDomainSpecType is the type of the VirtualMachineDomainSpec resource.
const VirtualMachineDomainSpecType = resource.Type("VirtualMachineDomainSpecs.hypervisor.talos.dev")

// VirtualMachineDomainSpec holds a rendered libvirt domain definition.
//
// The ID is the virtual machine name. Publishing a spec does not define or start a domain.
type VirtualMachineDomainSpec = typed.Resource[VirtualMachineDomainSpecSpec, VirtualMachineDomainSpecExtension]

// VirtualMachineDomainSpecSpec is the spec for VirtualMachineDomainSpec.
//
//gotagsrewrite:gen
type VirtualMachineDomainSpecSpec struct {
	// DomainXML is a libvirt domain definition, suitable for DomainDefineXML.
	DomainXML string `yaml:"domainXML" protobuf:"1"`
}

// NewVirtualMachineDomainSpec initializes a VirtualMachineDomainSpec resource.
func NewVirtualMachineDomainSpec(namespace resource.Namespace, id resource.ID) *VirtualMachineDomainSpec {
	return typed.NewResource[VirtualMachineDomainSpecSpec, VirtualMachineDomainSpecExtension](
		resource.NewMetadata(namespace, VirtualMachineDomainSpecType, id, resource.VersionUndefined),
		VirtualMachineDomainSpecSpec{},
	)
}

// VirtualMachineDomainSpecExtension is auxiliary resource data for VirtualMachineDomainSpec.
type VirtualMachineDomainSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VirtualMachineDomainSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineDomainSpecType,
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineDomainSpecType, &VirtualMachineDomainSpec{}); err != nil {
		panic(err)
	}
}
