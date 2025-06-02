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

// DiscoveredVolumeType is type of DiscoveredVolume resource.
const DiscoveredVolumeType = resource.Type("DiscoveredVolumes.block.talos.dev")

// DiscoveredVolume resource holds status of hardware DiscoveredVolumes (overall).
type DiscoveredVolume = typed.Resource[DiscoveredVolumeSpec, DiscoveredVolumeExtension]

// DiscoveredVolumeSpec is the spec for DiscoveredVolumes resource.
//
//gotagsrewrite:gen
type DiscoveredVolumeSpec struct {
	DevPath       string `yaml:"dev_path" protobuf:"17"`
	Type          string `yaml:"type" protobuf:"14"`
	DevicePath    string `yaml:"device_path" protobuf:"15"`
	Parent        string `yaml:"parent,omitempty" protobuf:"16"`
	ParentDevPath string `yaml:"parent_dev_path,omitempty" protobuf:"18"`

	// Overall size of the probed device (in bytes).
	Size       uint64 `yaml:"size" protobuf:"1"`
	PrettySize string `yaml:"pretty_size" protobuf:"19"`

	// Sector size of the device (in bytes).
	SectorSize uint `yaml:"sector_size,omitempty" protobuf:"2"`

	// Optimal I/O size for the device (in bytes).
	IOSize uint `yaml:"io_size,omitempty" protobuf:"3"`

	Name  string `yaml:"name" protobuf:"4"`
	UUID  string `yaml:"uuid,omitempty" protobuf:"5"`
	Label string `yaml:"label,omitempty" protobuf:"6"`

	BlockSize           uint32 `yaml:"block_size,omitempty" protobuf:"7"`
	FilesystemBlockSize uint32 `yaml:"filesystem_block_size,omitempty" protobuf:"8"`
	ProbedSize          uint64 `yaml:"probed_size,omitempty" protobuf:"9"`

	PartitionUUID  string `yaml:"partition_uuid,omitempty" protobuf:"10"`
	PartitionType  string `yaml:"partition_type,omitempty" protobuf:"11"`
	PartitionLabel string `yaml:"partition_label,omitempty" protobuf:"12"`
	PartitionIndex uint   `yaml:"partition_index,omitempty" protobuf:"13"`
}

// SetSize sets the size of the DiscoveredVolume, including the pretty size.
func (s *DiscoveredVolumeSpec) SetSize(size uint64) {
	s.Size = size
	s.PrettySize = humanize.Bytes(size)
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
				JSONPath: `{.pretty_size}`,
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
				JSONPath: `{.partition_label}`,
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
