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
	// Phase reports whether the source may be attached or observation is unavailable.
	Phase VirtualMachineDiskPhase `yaml:"phase" protobuf:"6"`
	// Error describes why the disk is not ready.
	Error string `yaml:"error,omitempty" protobuf:"7"`
	// Image is the content library image this status resolved.
	Image VirtualMachineDiskFromImageSpec `yaml:"image,omitempty" protobuf:"8"`
	// Pool is the storage pool the disk's volume lives in, for a disk provisioned into one.
	//
	// Stamped whether or not the disk resolved, so a failed one still names what it was for. That
	// is what lets a pool a running guest is reading from be recognized as in use: a status which
	// did not resolve has no SourcePath to go on.
	Pool string `yaml:"pool,omitempty" protobuf:"9"`
	// Volume is the name of that volume within the pool.
	//
	// Stamped whether or not the disk resolved, for the same reason as Pool.
	Volume string `yaml:"volume,omitempty" protobuf:"10"`
	// Size is the volume's actual logical capacity in bytes.
	//
	// It may exceed the size configured: a volume is never shrunk.
	Size uint64 `yaml:"size,omitempty" protobuf:"11"`
	// Blank is true when the disk was provisioned as an empty volume.
	Blank bool `yaml:"blank,omitempty" protobuf:"12"`
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
	var parts []string

	if disk.Provision.Blank {
		// Pool and format are in, because each changes the file the domain is pointed at -- and
		// format changes its driver type too -- so a running domain must be re-rendered onto a new
		// status rather than silently retargeted.
		//
		// Size is deliberately out. The volume's file name cannot depend on it, or growing a volume
		// would create a fresh empty one beside it, so a size in the ID would put two statuses on
		// one file at two capacities with no way to say which wins. Keeping it out makes a resize a
		// plain update of the status a running domain already holds, which is legal, and which the
		// domain XML -- carrying no capacity at all -- needs no change for.
		//
		// "provision=blank" cannot collide with the image branch below: '=' is outside the charset
		// a library name is validated against.
		parts = []string{"provision=blank", disk.Pool, disk.Format}
	} else {
		var image VirtualMachineDiskFromImageSpec

		if disk.Provision.FromImage != nil {
			image = *disk.Provision.FromImage
		}

		// A disk provisioned from neither hashes the zero value, so it still has one stable ID to
		// report its own unsupportedness under.
		parts = []string{image.Library, image.File, image.Digest, image.Mode}
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))

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
				Name:     "Pool",
				JSONPath: `{.pool}`,
			},
			{
				Name:     "Phase",
				JSONPath: `{.phase}`,
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
