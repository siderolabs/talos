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

// VirtualMachineDomainStatusType identifies libvirt domain observations, including unmanaged domains.
const VirtualMachineDomainStatusType = resource.Type("VirtualMachineDomainStatuses.hypervisor.talos.dev")

// VirtualMachineDomainStatus reports one domain present in libvirt, regardless of owner.
type VirtualMachineDomainStatus = typed.Resource[VirtualMachineDomainStatusSpec, VirtualMachineDomainStatusExtension]

// VirtualMachineDomainStatusSpec describes a domain as observed in libvirt.
//
//gotagsrewrite:gen
type VirtualMachineDomainStatusSpec struct {
	UUID         string                   `yaml:"uuid" protobuf:"1"`
	PowerState   VirtualMachinePowerState `yaml:"powerState" protobuf:"2"`
	Error        string                   `yaml:"error,omitempty" protobuf:"3"`
	State        uint32                   `yaml:"state" protobuf:"4"`
	MaxMemoryKiB uint64                   `yaml:"maxMemoryKiB" protobuf:"5"`
	MemoryKiB    uint64                   `yaml:"memoryKiB" protobuf:"6"`
	VCPUs        uint32                   `yaml:"vCPUs" protobuf:"7"`
}

// NewVirtualMachineDomainStatus initializes a domain observation.
func NewVirtualMachineDomainStatus(namespace resource.Namespace, id resource.ID) *VirtualMachineDomainStatus {
	return typed.NewResource[VirtualMachineDomainStatusSpec, VirtualMachineDomainStatusExtension](
		resource.NewMetadata(namespace, VirtualMachineDomainStatusType, id, resource.VersionUndefined),
		VirtualMachineDomainStatusSpec{},
	)
}

// VirtualMachineDomainStatusExtension supplies the resource definition.
type VirtualMachineDomainStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (VirtualMachineDomainStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineDomainStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "UUID", JSONPath: "{.uuid}"},
			{Name: "Power", JSONPath: "{.powerState}"},
			{Name: "State", JSONPath: "{.state}"},
			{Name: "Memory (KiB)", JSONPath: "{.memoryKiB}"},
			{Name: "VCPUs", JSONPath: "{.vCPUs}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineDomainStatusType, &VirtualMachineDomainStatus{}); err != nil {
		panic(err)
	}
}
