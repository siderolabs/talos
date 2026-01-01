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

// PhysicalVolumeType is type of PhysicalVolume resource.
const PhysicalVolumeType = resource.Type("PhysicalVolumes.block.talos.dev")

// PhysicalVolume represents an LVM physical volume resource.
type PhysicalVolume = typed.Resource[PhysicalVolumeSpec, PhysicalVolumeExtension]

// PhysicalVolumeSpec describes an LVM Physical Volume.
//
//gotagsrewrite:gen
type PhysicalVolumeSpec struct {
	DevicePath       string `yaml:"devicePath" protobuf:"1"`
	VGName           string `yaml:"vgName" protobuf:"2"`
	Size             uint64 `yaml:"size" protobuf:"3"`
	UUID             string `yaml:"uuid" protobuf:"4"`
	AllocatedExtents uint64 `yaml:"allocatedExtents,omitempty" protobuf:"5"`
	TotalExtents     uint64 `yaml:"totalExtents,omitempty" protobuf:"6"`
}

// NewPhysicalVolume initializes a PhysicalVolume resource.
func NewPhysicalVolume(namespace resource.Namespace, id resource.ID) *PhysicalVolume {
	return typed.NewResource[PhysicalVolumeSpec, PhysicalVolumeExtension](
		resource.NewMetadata(namespace, PhysicalVolumeType, id, resource.VersionUndefined),
		PhysicalVolumeSpec{},
	)
}

// PhysicalVolumeExtension provides auxiliary methods for PhysicalVolume.
type PhysicalVolumeExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (PhysicalVolumeExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PhysicalVolumeType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Device",
				JSONPath: `{.devicePath}`,
			},
			{
				Name:     "VG",
				JSONPath: `{.vgName}`,
			},
			{
				Name:     "Size",
				JSONPath: `{.size}`,
			},
			{
				Name:     "UUID",
				JSONPath: `{.uuid}`,
			},
			{
				Name:     "Allocated",
				JSONPath: `{.allocatedExtents}`,
			},
			{
				Name:     "Total",
				JSONPath: `{.totalExtents}`,
			},
		},
	}
}

// VolumeGroupType is type of VolumeGroup resource.
const VolumeGroupType = resource.Type("VolumeGroups.block.talos.dev")

// VolumeGroup represents an LVM volume group resource.
type VolumeGroup = typed.Resource[VolumeGroupSpec, VolumeGroupExtension]

// VolumeGroupSpec describes an LVM Volume Group.
//
//gotagsrewrite:gen
type VolumeGroupSpec struct {
	Name            string   `yaml:"name" protobuf:"1"`
	PhysicalVolumes []string `yaml:"physicalVolumes" protobuf:"2"`
	TotalSize       uint64   `yaml:"totalSize" protobuf:"3"`
	FreeSize        uint64   `yaml:"freeSize" protobuf:"4"`
	UUID            string   `yaml:"uuid" protobuf:"5"`
	ExtentSize      uint64   `yaml:"extentSize,omitempty" protobuf:"6"`
	LVCount         uint32   `yaml:"lvCount,omitempty" protobuf:"7"`
	PVCount         uint32   `yaml:"pvCount,omitempty" protobuf:"8"`
}

// NewVolumeGroup initializes a VolumeGroup resource.
func NewVolumeGroup(namespace resource.Namespace, id resource.ID) *VolumeGroup {
	return typed.NewResource[VolumeGroupSpec, VolumeGroupExtension](
		resource.NewMetadata(namespace, VolumeGroupType, id, resource.VersionUndefined),
		VolumeGroupSpec{},
	)
}

// VolumeGroupExtension provides auxiliary methods for VolumeGroup.
type VolumeGroupExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeGroupExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeGroupType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: `{.name}`,
			},
			{
				Name:     "UUID",
				JSONPath: `{.uuid}`,
			},
			{
				Name:     "TotalSize",
				JSONPath: `{.totalSize}`,
			},
			{
				Name:     "FreeSize",
				JSONPath: `{.freeSize}`,
			},
			{
				Name:     "LVCount",
				JSONPath: `{.lvCount}`,
			},
			{
				Name:     "PVCount",
				JSONPath: `{.pvCount}`,
			},
		},
	}
}

// LogicalVolumeType is type of LogicalVolume resource.
const LogicalVolumeType = resource.Type("LogicalVolumes.block.talos.dev")

// LVType describes the type of logical volume (linear, striped, mirror).
type LVType int

// Logical volume types.
//
//structprotogen:gen_enum
const (
	LVTypeLinear  LVType = iota // linear
	LVTypeStriped               // striped
	LVTypeMirror                // mirror
)

// LogicalVolume represents an LVM logical volume resource.
type LogicalVolume = typed.Resource[LogicalVolumeSpec, LogicalVolumeExtension]

// LogicalVolumeSpec describes an LVM Logical Volume.
//
//gotagsrewrite:gen
type LogicalVolumeSpec struct {
	Name       string `yaml:"name" protobuf:"1"`
	VGName     string `yaml:"vgName" protobuf:"2"`
	Size       uint64 `yaml:"size" protobuf:"3"`
	Type       LVType `yaml:"type" protobuf:"4"`
	Stripes    int    `yaml:"stripes,omitempty" protobuf:"5"`
	Mirrors    int    `yaml:"mirrors,omitempty" protobuf:"6"`
	UUID       string `yaml:"uuid,omitempty" protobuf:"7"`
	DevicePath string `yaml:"devicePath,omitempty" protobuf:"8"`
	Symlink    string `yaml:"symlink,omitempty" protobuf:"9"`
	State      string `yaml:"state,omitempty" protobuf:"10"`
}

// NewLogicalVolume initializes a LogicalVolume resource.
func NewLogicalVolume(namespace resource.Namespace, id resource.ID) *LogicalVolume {
	return typed.NewResource[LogicalVolumeSpec, LogicalVolumeExtension](
		resource.NewMetadata(namespace, LogicalVolumeType, id, resource.VersionUndefined),
		LogicalVolumeSpec{},
	)
}

// LogicalVolumeExtension provides auxiliary methods for LogicalVolume.
type LogicalVolumeExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LogicalVolumeExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LogicalVolumeType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: `{.name}`,
			},
			{
				Name:     "VG",
				JSONPath: `{.vgName}`,
			},
			{
				Name:     "Size",
				JSONPath: `{.size}`,
			},
			{
				Name:     "Type",
				JSONPath: `{.type}`,
			},
			{
				Name:     "UUID",
				JSONPath: `{.uuid}`,
			},
			{
				Name:     "Device",
				JSONPath: `{.devicePath}`,
			},
			{
				Name:     "State",
				JSONPath: `{.state}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[PhysicalVolumeSpec](PhysicalVolumeType, &PhysicalVolume{}); err != nil {
		panic(err)
	}

	if err := protobuf.RegisterDynamic[VolumeGroupSpec](VolumeGroupType, &VolumeGroup{}); err != nil {
		panic(err)
	}

	if err := protobuf.RegisterDynamic[LogicalVolumeSpec](LogicalVolumeType, &LogicalVolume{}); err != nil {
		panic(err)
	}
}
