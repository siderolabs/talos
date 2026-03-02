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

// LVMLogicalVolumeType is type of LVMLogicalVolume resource.
const LVMLogicalVolumeType = resource.Type("LVMLogicalVolumes.block.talos.dev")

// LVType describes the type of logical volume.
type LVType int

// Logical volume types.
//
//structprotogen:gen_enum
const (
	LVTypeLinear LVType = iota // linear
)

// LVMLogicalVolume represents an LVM logical volume resource.
type LVMLogicalVolume = typed.Resource[LVMLogicalVolumeSpec, LVMLogicalVolumeExtension]

// LVMLogicalVolumeSpec describes an LVM Logical Volume.
//
//gotagsrewrite:gen
type LVMLogicalVolumeSpec struct {
	Name            string `yaml:"name" protobuf:"1"`
	VolumeGroupName string `yaml:"volumeGroupName" protobuf:"2"`
	Size            uint64 `yaml:"size" protobuf:"3"`
	Type            LVType `yaml:"type" protobuf:"4"`
	UUID            string `yaml:"uuid,omitempty" protobuf:"7"`
	DevicePath      string `yaml:"devicePath,omitempty" protobuf:"8"`
	Symlink         string `yaml:"symlink,omitempty" protobuf:"9"`
	State           string `yaml:"state,omitempty" protobuf:"10"`
}

// NewLVMLogicalVolume initializes a LVMLogicalVolume resource.
func NewLVMLogicalVolume(namespace resource.Namespace, id resource.ID) *LVMLogicalVolume {
	return typed.NewResource[LVMLogicalVolumeSpec, LVMLogicalVolumeExtension](
		resource.NewMetadata(namespace, LVMLogicalVolumeType, id, resource.VersionUndefined),
		LVMLogicalVolumeSpec{},
	)
}

// LVMLogicalVolumeExtension provides auxiliary methods for LVMLogicalVolume.
type LVMLogicalVolumeExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMLogicalVolumeExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMLogicalVolumeType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: `{.name}`,
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

	if err := protobuf.RegisterDynamic(LVMLogicalVolumeType, &LVMLogicalVolume{}); err != nil {
		panic(err)
	}
}
