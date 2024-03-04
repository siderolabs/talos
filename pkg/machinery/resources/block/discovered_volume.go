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

// DiscoveredVolumeType is type of BlockDiscoveredVolume resource.
const DiscoveredVolumeType = resource.Type("DiscoveredVolumes.block.talos.dev")

// DiscoveredVolume resource holds status of hardware DiscoveredVolumes (overall).
type DiscoveredVolume = typed.Resource[DiscoveredVolumeSpec, DiscoveredVolumeExtension]

// DiscoveredVolumeSpec is the spec for DiscoveredVolumes status.
//
//gotagsrewrite:gen
type DiscoveredVolumeSpec struct {
	Type       string `yaml:"type" protobuf:"14"`
	DevicePath string `yaml:"devicePath" protobuf:"15"`
	Parent     string `yaml:"parent,omitempty" protobuf:"16"`

	// Overall size of the probed device (in bytes).
	Size uint64 `yaml:"size" protobuf:"1"`

	// Sector size of the device (in bytes).
	SectorSize uint `yaml:"sectorSize,omitempty" protobuf:"2"`

	// Optimal I/O size for the device (in bytes).
	IOSize uint `yaml:"ioSize,omitempty" protobuf:"3"`

	Name  string `yaml:"name" protobuf:"4"`
	UUID  string `yaml:"uuid,omitempty" protobuf:"5"`
	Label string `yaml:"label,omitempty" protobuf:"6"`

	BlockSize           uint32 `yaml:"blockSize,omitempty" protobuf:"7"`
	FilesystemBlockSize uint32 `yaml:"filesystemBlockSize,omitempty" protobuf:"8"`
	ProbedSize          uint64 `yaml:"probedSize,omitempty" protobuf:"9"`

	PartitionUUID  string `yaml:"partitionUUID,omitempty" protobuf:"10"`
	PartitionType  string `yaml:"partitionType,omitempty" protobuf:"11"`
	PartitionLabel string `yaml:"partitionLabel,omitempty" protobuf:"12"`
	PartitionIndex uint   `yaml:"partitionIndex,omitempty" protobuf:"13"`
}

// NewDiscoveredVolume initializes a BlockDiscoveredVolume resource.
func NewDiscoveredVolume(namespace resource.Namespace, id resource.ID) *DiscoveredVolume {
	return typed.NewResource[DiscoveredVolumeSpec, DiscoveredVolumeExtension](
		resource.NewMetadata(namespace, DiscoveredVolumeType, id, resource.VersionUndefined),
		DiscoveredVolumeSpec{},
	)
}

// DiscoveredVolumeExtension is auxiliary resource data for BlockDiscoveredVolume.
type DiscoveredVolumeExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DiscoveredVolumeExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiscoveredVolumeType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Type",
				JSONPath: `{.type}`,
			},
			{
				Name:     "Size",
				JSONPath: `{.size}`,
			},
			{
				Name:     "Discovered",
				JSONPath: `{.name}`,
			},
			{
				Name:     "Label",
				JSONPath: `{.label}`,
			},
			{
				Name:     "PartitionLabel",
				JSONPath: `{.partitionLabel}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[DiscoveredVolumeSpec](DiscoveredVolumeType, &DiscoveredVolume{})
	if err != nil {
		panic(err)
	}
}
