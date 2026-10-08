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
type VirtualMachineDomainSpec = typed.Resource[VirtualMachineDomainSpecSpec, VirtualMachineDomainSpecExtension]

// VirtualMachineDomainSpecSpec is the spec for VirtualMachineDomainSpec.
//
//gotagsrewrite:gen
type VirtualMachineDomainSpecSpec struct {
	// DomainXML is the libvirt domain description used to start the guest.
	DomainXML string `yaml:"domainXML" protobuf:"1"`
	// PowerState selects running or stopped transient-domain behavior.
	PowerState string `yaml:"powerState" protobuf:"2"`
	// Disks lists the IDs of the VirtualMachineDiskStatus resources DomainXML attaches.
	//
	// It is what the controller which starts the domain holds against, rather than the virtual
	// machine's configuration, which moves ahead of the definition libvirt is running.
	Disks []string `yaml:"disks,omitempty" protobuf:"3"`
	// CloudInit identifies the seed status this domain has attached and must hold.
	CloudInit string `yaml:"cloudInit,omitempty" protobuf:"4"`
	// ObservedGeneration identifies the VM intent used to render this definition.
	ObservedGeneration string `yaml:"observedGeneration,omitempty" protobuf:"5"`
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
