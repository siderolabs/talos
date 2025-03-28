// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/dustin/go-humanize"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VolumeStatusType is type of VolumeStatus resource.
const VolumeStatusType = resource.Type("VolumeStatuses.block.talos.dev")

// VolumeStatus resource contains information about the volume status.
type VolumeStatus = typed.Resource[VolumeStatusSpec, VolumeStatusExtension]

// VolumeStatusSpec is the spec for VolumeStatus resource.
//
//gotagsrewrite:gen
type VolumeStatusSpec struct {
	Phase        VolumePhase `yaml:"phase" protobuf:"1"`
	PreFailPhase VolumePhase `yaml:"preFailPhase,omitempty" protobuf:"6"`

	Type     VolumeType `yaml:"type" protobuf:"16"`
	ParentID string     `yaml:"parentID,omitempty" protobuf:"19"`

	// Location is the path to the block device (raw).
	Location string `yaml:"location,omitempty" protobuf:"2"`
	// MountLocation is the location to be mounted, might be different from location.
	MountLocation string `yaml:"mountLocation,omitempty" protobuf:"11"`

	PartitionIndex int `yaml:"partitionIndex,omitempty" protobuf:"8"`

	// ParentLocation (if present) is the location of the parent block device for partitions.
	ParentLocation string `yaml:"parentLocation,omitempty" protobuf:"7"`
	UUID           string `yaml:"uuid,omitempty" protobuf:"4"`
	PartitionUUID  string `yaml:"partitionUUID,omitempty" protobuf:"5"`
	Size           uint64 `yaml:"size,omitempty" protobuf:"9"`
	PrettySize     string `yaml:"prettySize,omitempty" protobuf:"13"`

	// Filesystem is the filesystem type.
	Filesystem FilesystemType `yaml:"filesystem,omitempty" protobuf:"10"`

	// EncryptionProvider is the provider of the encryption which was used to unlock the volume.
	EncryptionProvider EncryptionProviderType `yaml:"encryptionProvider,omitempty" protobuf:"12"`
	// EncryptionFailedSyncs is the list of failed syncs for the volume (per key/provider)/
	EncryptionFailedSyncs []string `yaml:"encryptionFailedSyncs,omitempty" protobuf:"14"`
	// ConfiguredEncryptionKeys is the list of configured encryption keys for the volume.
	ConfiguredEncryptionKeys []string `yaml:"configuredEncryptionKeys,omitempty" protobuf:"17"`

	// MountSpec is the mount specification.
	MountSpec MountSpec `yaml:"mountSpec,omitempty" protobuf:"15"`

	// Symlink is the symlink specification.
	SymlinkSpec SymlinkProvisioningSpec `yaml:"symlink,omitempty" protobuf:"18"`

	ErrorMessage string `yaml:"errorMessage,omitempty" protobuf:"3"`
}

// SetSize sets the size of the volume status, including the pretty size.
func (s *VolumeStatusSpec) SetSize(size uint64) {
	s.Size = size
	s.PrettySize = humanize.Bytes(size)
}

// NewVolumeStatus initializes a BlockVolumeStatus resource.
func NewVolumeStatus(namespace resource.Namespace, id resource.ID) *VolumeStatus {
	return typed.NewResource[VolumeStatusSpec, VolumeStatusExtension](
		resource.NewMetadata(namespace, VolumeStatusType, id, resource.VersionUndefined),
		VolumeStatusSpec{},
	)
}

// VolumeStatusExtension is auxiliary resource data for BlockVolumeStatus.
type VolumeStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Type",
				JSONPath: `{.type}`,
			},
			{
				Name:     "Phase",
				JSONPath: `{.phase}`,
			},
			{
				Name:     "Location",
				JSONPath: `{.location}`,
			},
			{
				Name:     "Size",
				JSONPath: `{.prettySize}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[VolumeStatusSpec](VolumeStatusType, &VolumeStatus{})
	if err != nil {
		panic(err)
	}
}
