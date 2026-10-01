// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VirtualMachineDiskStatusType is the type of the VirtualMachineDiskStatus resource.
const VirtualMachineDiskStatusType = resource.Type("VirtualMachineDiskStatuses.hypervisor.talos.dev")

// VirtualMachineDiskStatus resolves a disk of a virtual machine to a host source libvirt can open.
//
// The ID is built by VirtualMachineDiskStatusID.
type VirtualMachineDiskStatus = typed.Resource[VirtualMachineDiskStatusSpec, VirtualMachineDiskStatusExtension]

// VirtualMachineDiskStatusSpec is the spec for VirtualMachineDiskStatus.
//
//gotagsrewrite:gen
type VirtualMachineDiskStatusSpec struct {
	// VirtualMachine is the name of the virtual machine the disk belongs to.
	VirtualMachine string `yaml:"virtualMachine" protobuf:"1"`
	// Name is the disk's name within that virtual machine.
	Name string `yaml:"name" protobuf:"2"`
	// SourcePath is the absolute host path libvirt opens.
	//
	// Only meaningful when Ready.
	SourcePath string `yaml:"sourcePath,omitempty" protobuf:"3"`
	// Format is the on-host format of SourcePath, as libvirt's disk driver type.
	//
	// Only meaningful when Ready.
	Format string `yaml:"format,omitempty" protobuf:"4"`
	// ReadOnly is true when the guest must not write to the source.
	ReadOnly bool `yaml:"readOnly" protobuf:"5"`
	// Ready is true once the source exists and may be attached.
	Ready bool `yaml:"ready" protobuf:"6"`
	// Error describes why the disk is not ready.
	Error string `yaml:"error,omitempty" protobuf:"7"`
	// Image is the content library image this status resolved.
	Image VirtualMachineDiskFromImageSpec `yaml:"image,omitempty" protobuf:"8"`
}

// diskStatusIDDigestLength is how much of the provisioning digest the ID carries. It only has to
// tell one configuration of a single disk apart from the next.
const diskStatusIDDigestLength = 12

// VirtualMachineDiskStatusID builds the resource ID for a disk of a virtual machine. Both names are
// validated against ^[A-Za-z0-9-]+$, so neither separator can occur within either.
//
// It digests what the disk is provisioned from as well as naming the disk: a finalizer stops a
// resource being destroyed, not updated, so re-provisioning from another image must create a second
// status rather than rewrite the one a running domain holds.
func VirtualMachineDiskStatusID(virtualMachine string, disk VirtualMachineDiskSpec) resource.ID {
	var image VirtualMachineDiskFromImageSpec

	if disk.Provision.FromImage != nil {
		image = *disk.Provision.FromImage
	}

	// A disk provisioned from no image hashes the zero value, so it still has one stable ID to
	// report its own unsupportedness under.
	sum := sha256.Sum256([]byte(strings.Join([]string{image.Library, image.File, image.Digest, image.Mode}, "\x00")))

	return virtualMachine + "/" + disk.Name + "@" + hex.EncodeToString(sum[:])[:diskStatusIDDigestLength]
}

// NewVirtualMachineDiskStatus initializes a VirtualMachineDiskStatus resource.
func NewVirtualMachineDiskStatus(namespace resource.Namespace, id resource.ID) *VirtualMachineDiskStatus {
	return typed.NewResource[VirtualMachineDiskStatusSpec, VirtualMachineDiskStatusExtension](
		resource.NewMetadata(namespace, VirtualMachineDiskStatusType, id, resource.VersionUndefined),
		VirtualMachineDiskStatusSpec{},
	)
}

// VirtualMachineDiskStatusExtension is auxiliary resource data for VirtualMachineDiskStatus.
type VirtualMachineDiskStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VirtualMachineDiskStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VirtualMachineDiskStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Machine",
				JSONPath: `{.virtualMachine}`,
			},
			{
				Name:     "Disk",
				JSONPath: `{.name}`,
			},
			{
				Name:     "Ready",
				JSONPath: `{.ready}`,
			},
			{
				Name:     "Source",
				JSONPath: `{.sourcePath}`,
			},
			{
				Name:     "Error",
				JSONPath: `{.error}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(VirtualMachineDiskStatusType, &VirtualMachineDiskStatus{}); err != nil {
		panic(err)
	}
}
