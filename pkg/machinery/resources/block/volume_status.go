// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

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

	// Filesystem is the filesystem type.
	Filesystem FilesystemType `yaml:"filesystem,omitempty" protobuf:"10"`

	// EncryptionProvider is the provider of the encryption.
	EncryptionProvider EncryptionProviderType `yaml:"encryptionProvider,omitempty" protobuf:"12"`

	ErrorMessage string `yaml:"errorMessage,omitempty" protobuf:"3"`
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
				Name:     "Phase",
				JSONPath: `{.phase}`,
			},
			{
				Name:     "Location",
				JSONPath: `{.location}`,
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
