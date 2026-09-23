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

// VirtualMachineSpecType is the type of the VirtualMachineSpec resource.
const VirtualMachineSpecType = resource.Type("VirtualMachineSpecs.hypervisor.talos.dev")

// VirtualMachineSpec holds the backend-neutral desired configuration of a virtual machine.
//
// The ID is the virtual machine name. Publishing a spec does not define or start a domain.
type VirtualMachineSpec = typed.Resource[VirtualMachineSpecSpec, VirtualMachineSpecExtension]

// VirtualMachineSpecSpec is the spec for VirtualMachineSpec.
//
//gotagsrewrite:gen
type VirtualMachineSpecSpec struct {
	CPU        VirtualMachineCPUSpec      `yaml:"cpu" protobuf:"1"`
	Memory     VirtualMachineMemorySpec   `yaml:"memory" protobuf:"2"`
	PowerState string                     `yaml:"powerState" protobuf:"3"`
	Firmware   VirtualMachineFirmwareSpec `yaml:"firmware" protobuf:"4"`
	Console    VirtualMachineConsoleSpec  `yaml:"console" protobuf:"5"`
	// Disks are logical volume intent, not libvirt source paths. Resolution and
	// provisioning belong to a storage controller, not the XML renderer.
	Disks []VirtualMachineDiskSpec `yaml:"disks,omitempty" protobuf:"6"`
}

// VirtualMachineFirmwareSpec describes firmware selection without host firmware paths.
//
//gotagsrewrite:gen
type VirtualMachineFirmwareSpec struct {
	Type       string `yaml:"type" protobuf:"1"`
	SecureBoot bool   `yaml:"secureBoot" protobuf:"2"`
}

// VirtualMachineConsoleSpec describes requested guest consoles.
//
//gotagsrewrite:gen
type VirtualMachineConsoleSpec struct {
	Serial bool `yaml:"serial" protobuf:"1"`
	VNC    bool `yaml:"vnc" protobuf:"2"`
}

// VirtualMachineDiskSpec describes a disk before its volume has a host source.
//
//gotagsrewrite:gen
type VirtualMachineDiskSpec struct {
	Name      string                          `yaml:"name" protobuf:"1"`
	Pool      string                          `yaml:"pool" protobuf:"2"`
	Size      uint64                          `yaml:"size" protobuf:"3"`
	Format    string                          `yaml:"format" protobuf:"4"`
	Bus       string                          `yaml:"bus" protobuf:"5"`
	Type      string                          `yaml:"type" protobuf:"6"`
	BootOrder uint32                          `yaml:"bootOrder" protobuf:"7"`
	Provision VirtualMachineDiskProvisionSpec `yaml:"provision" protobuf:"8"`
}

// VirtualMachineDiskProvisionSpec names exactly one volume-content source.
//
//gotagsrewrite:gen
type VirtualMachineDiskProvisionSpec struct {
	Blank     bool                             `yaml:"blank" protobuf:"1"`
	FromImage *VirtualMachineDiskFromImageSpec `yaml:"fromImage,omitempty" protobuf:"2"`
}

// VirtualMachineDiskFromImageSpec identifies an image in a content library.
//
//gotagsrewrite:gen
type VirtualMachineDiskFromImageSpec struct {
	Library string `yaml:"library" protobuf:"1"`
	File    string `yaml:"file" protobuf:"2"`
	Digest  string `yaml:"digest" protobuf:"3"`
	Mode    string `yaml:"mode" protobuf:"4"`
}

// VirtualMachineCPUSpec describes the desired virtual CPUs.
//
//gotagsrewrite:gen
type VirtualMachineCPUSpec struct {
	Count uint32 `yaml:"count" protobuf:"1"`
}

// VirtualMachineMemorySpec describes the desired guest memory.
//
//gotagsrewrite:gen
type VirtualMachineMemorySpec struct {
	// Size is the guest memory size in bytes.
	Size       uint64                             `yaml:"size" protobuf:"1"`
	Ballooning VirtualMachineMemoryBallooningSpec `yaml:"ballooning" protobuf:"2"`
}

// VirtualMachineMemoryBallooningSpec describes the desired memory ballooning state.
//
//gotagsrewrite:gen
type VirtualMachineMemoryBallooningSpec struct {
	Enabled bool `yaml:"enabled" protobuf:"1"`
}

// NewVirtualMachineSpec initializes a VirtualMachineSpec resource.
func NewVirtualMachineSpec(namespace resource.Namespace, id resource.ID) *VirtualMachineSpec {
	return typed.NewResource[VirtualMachineSpecSpec, VirtualMachineSpecExtension](
		resource.NewMetadata(namespace, VirtualMachineSpecType, id, resource.VersionUndefined),
		VirtualMachineSpecSpec{},
	)
}

// VirtualMachineSpecExtension is auxiliary resource data for VirtualMachineSpec.
type VirtualMachineSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VirtualMachineSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineSpecType,
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineSpecType, &VirtualMachineSpec{}); err != nil {
		panic(err)
	}
}
