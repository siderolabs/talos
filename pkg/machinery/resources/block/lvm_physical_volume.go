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

// LVMPhysicalVolumeType is type of LVMPhysicalVolume resource.
const LVMPhysicalVolumeType = resource.Type("LVMPhysicalVolumes.block.talos.dev")

// LVMPhysicalVolume represents an LVM physical volume resource.
type LVMPhysicalVolume = typed.Resource[LVMPhysicalVolumeSpec, LVMPhysicalVolumeExtension]

// LVMPhysicalVolumeSpec describes an LVM Physical Volume.
//
//gotagsrewrite:gen
type LVMPhysicalVolumeSpec struct {
	DevicePath       string `yaml:"devicePath" protobuf:"1"`
	VolumeGroupName  string `yaml:"volumeGroupName" protobuf:"2"`
	Size             uint64 `yaml:"size" protobuf:"3"`
	UUID             string `yaml:"uuid" protobuf:"4"`
	AllocatedExtents uint64 `yaml:"allocatedExtents,omitempty" protobuf:"5"`
	TotalExtents     uint64 `yaml:"totalExtents,omitempty" protobuf:"6"`
}

// NewLVMPhysicalVolume initializes a LVMPhysicalVolume resource.
func NewLVMPhysicalVolume(namespace resource.Namespace, id resource.ID) *LVMPhysicalVolume {
	return typed.NewResource[LVMPhysicalVolumeSpec, LVMPhysicalVolumeExtension](
		resource.NewMetadata(namespace, LVMPhysicalVolumeType, id, resource.VersionUndefined),
		LVMPhysicalVolumeSpec{},
	)
}

// LVMPhysicalVolumeExtension provides auxiliary methods for LVMPhysicalVolume.
type LVMPhysicalVolumeExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMPhysicalVolumeExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMPhysicalVolumeType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Device",
				JSONPath: `{.devicePath}`,
			},
			{
				Name:     "VG",
				JSONPath: `{.volumeGroupName}`,
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

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMPhysicalVolumeType, &LVMPhysicalVolume{}); err != nil {
		panic(err)
	}
}
