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

// LVMVolumeGroupType is type of LVMVolumeGroup resource.
const LVMVolumeGroupType = resource.Type("LVMVolumeGroups.block.talos.dev")

// LVMVolumeGroup represents an LVM volume group resource.
type LVMVolumeGroup = typed.Resource[LVMVolumeGroupSpec, LVMVolumeGroupExtension]

// LVMVolumeGroupSpec describes an LVM Volume Group.
//
//gotagsrewrite:gen
type LVMVolumeGroupSpec struct {
	Name                 string   `yaml:"name" protobuf:"1"`
	PhysicalVolumes      []string `yaml:"physicalVolumes" protobuf:"2"`
	TotalSize            uint64   `yaml:"totalSize" protobuf:"3"`
	FreeSize             uint64   `yaml:"freeSize" protobuf:"4"`
	UUID                 string   `yaml:"uuid" protobuf:"5"`
	ExtentSize           uint64   `yaml:"extentSize,omitempty" protobuf:"6"`
	LogicalVolumesCount  uint32   `yaml:"logicalVolumesCount,omitempty" protobuf:"7"`
	PhysicalVolumesCount uint32   `yaml:"physicalVolumesCount,omitempty" protobuf:"8"`
}

// NewLVMVolumeGroup initializes a LVMVolumeGroup resource.
func NewLVMVolumeGroup(namespace resource.Namespace, id resource.ID) *LVMVolumeGroup {
	return typed.NewResource[LVMVolumeGroupSpec, LVMVolumeGroupExtension](
		resource.NewMetadata(namespace, LVMVolumeGroupType, id, resource.VersionUndefined),
		LVMVolumeGroupSpec{},
	)
}

// LVMVolumeGroupExtension provides auxiliary methods for LVMVolumeGroup.
type LVMVolumeGroupExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMVolumeGroupExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMVolumeGroupType,
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
				JSONPath: `{.logicalVolumesCount}`,
			},
			{
				Name:     "PVCount",
				JSONPath: `{.physicalVolumesCount}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMVolumeGroupType, &LVMVolumeGroup{}); err != nil {
		panic(err)
	}
}
