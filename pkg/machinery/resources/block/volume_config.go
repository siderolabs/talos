// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VolumeConfigType is type of VolumeConfig resource.
const VolumeConfigType = resource.Type("VolumeConfigs.block.talos.dev")

// VolumeConfig resource contains configuration for machine volumes.
type VolumeConfig = typed.Resource[VolumeConfigSpec, VolumeConfigExtension]

// VolumeConfigSpec is the spec for VolumeConfig resource.
//
//gotagsrewrite:gen
type VolumeConfigSpec struct {
	// Parent volume ID, if set no operations on the volume continue until the parent volume is ready.
	ParentID string `yaml:"parentId,omitempty" protobuf:"1"`

	// Volume type.
	Type VolumeType `yaml:"type" protobuf:"2"`

	// Provisioning configuration (how to provision a volume).
	Provisioning ProvisioningSpec `yaml:"provisioning" protobuf:"3"`

	// How to find a volume.
	Locator LocatorSpec `yaml:"locator" protobuf:"4"`
}

// Wave constants.
const (
	WaveSystemDisk = -1
	WaveUserDisks  = 0
)

// ProvisioningSpec is the spec for volume provisioning.
//
//gotagsrewrite:gen
type ProvisioningSpec struct {
	// Provisioning wave for the volume.
	//
	// Waves are processed sequentially - the volumes in the wave are only provisioned after the previous wave is done.
	Wave int `yaml:"wave,omitempty" protobuf:"3"`

	// DiskSelector selects a disk for the volume.
	DiskSelector DiskSelector `yaml:"diskSelector,omitempty" protobuf:"1"`

	// PartitionSpec describes how to provision the volume (partition type).
	PartitionSpec PartitionSpec `yaml:"partitionSpec,omitempty" protobuf:"2"`

	// FilesystemSpec describes how to provision the volume (filesystem type).
	FilesystemSpec FilesystemSpec `yaml:"filesystemSpec,omitempty" protobuf:"4"`
}

// DiskSelector selects a disk for the volume.
//
//gotagsrewrite:gen
type DiskSelector struct {
	Match cel.Expression `yaml:"match,omitempty" protobuf:"1"`
}

// PartitionSpec is the spec for volume partitioning.
//
//gotagsrewrite:gen
type PartitionSpec struct {
	// Partition minimum size in bytes.
	MinSize uint64 `yaml:"minSize" protobuf:"1"`

	// Partition maximum size in bytes, if not set, grows to the maximum size.
	MaxSize uint64 `yaml:"maxSize,omitempty" protobuf:"2"`

	// Grow the partition automatically to the maximum size.
	Grow bool `yaml:"grow" protobuf:"3"`

	// Label for the partition.
	Label string `yaml:"label,omitempty" protobuf:"4"`

	// Partition type UUID.
	TypeUUID string `yaml:"typeUUID,omitempty" protobuf:"5"`
}

// LocatorSpec is the spec for volume locator.
//
//gotagsrewrite:gen
type LocatorSpec struct {
	Match cel.Expression `yaml:"match,omitempty" protobuf:"1"`
}

// FilesystemSpec is the spec for volume filesystem.
//
//gotagsrewrite:gen
type FilesystemSpec struct {
	// Filesystem type.
	Type FilesystemType `yaml:"type" protobuf:"1"`
}

// NewVolumeConfig initializes a BlockVolumeConfig resource.
func NewVolumeConfig(namespace resource.Namespace, id resource.ID) *VolumeConfig {
	return typed.NewResource[VolumeConfigSpec, VolumeConfigExtension](
		resource.NewMetadata(namespace, VolumeConfigType, id, resource.VersionUndefined),
		VolumeConfigSpec{},
	)
}

// VolumeConfigExtension is auxiliary resource data for BlockVolumeConfig.
type VolumeConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[VolumeConfigSpec](VolumeConfigType, &VolumeConfig{})
	if err != nil {
		panic(err)
	}
}
