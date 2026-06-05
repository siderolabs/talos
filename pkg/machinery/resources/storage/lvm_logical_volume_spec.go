// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LVMLogicalVolumeSpecType is the type of LVMLogicalVolumeSpec resource.
const LVMLogicalVolumeSpecType = resource.Type("LVMLogicalVolumeSpecs.storage.talos.dev")

// LVMLogicalVolumeSpec is the desired state for a logical volume.
type LVMLogicalVolumeSpec = typed.Resource[LVMLogicalVolumeSpecSpec, LVMLogicalVolumeSpecExtension]

// LVMLogicalVolumeSpecSpec is the spec for LVMLogicalVolumeSpec resource.
//
//gotagsrewrite:gen
type LVMLogicalVolumeSpecSpec struct {
	// VGName is the parent volume group name.
	VGName string `yaml:"vgName" protobuf:"1"`
	// Name is the logical volume name.
	Name string `yaml:"name" protobuf:"2"`
	// Type is the LV layout.
	Type LVMLogicalVolumeType `yaml:"type" protobuf:"3"`
	// SizeBytes is the absolute LV size in bytes; used when SizePercentVG is zero.
	SizeBytes uint64 `yaml:"sizeBytes" protobuf:"4"`
	// SizePercentVG, when non-zero, sizes the LV as a percentage of the VG.
	SizePercentVG uint32 `yaml:"sizePercentVG" protobuf:"5"`
	// Mirrors is the mirror count for raid1/raid10 layouts.
	Mirrors uint32 `yaml:"mirrors" protobuf:"6"`
	// Stripes is the stripe count for raid0/raid10 layouts; 0 means "all PVs",
	// resolved by the reconcile controller.
	Stripes uint32 `yaml:"stripes" protobuf:"7"`
}

// NewLVMLogicalVolumeSpec initializes a LVMLogicalVolumeSpec resource.
func NewLVMLogicalVolumeSpec(namespace resource.Namespace, id resource.ID) *LVMLogicalVolumeSpec {
	return typed.NewResource[LVMLogicalVolumeSpecSpec, LVMLogicalVolumeSpecExtension](
		resource.NewMetadata(namespace, LVMLogicalVolumeSpecType, id, resource.VersionUndefined),
		LVMLogicalVolumeSpecSpec{},
	)
}

// LVMLogicalVolumeSpecExtension is auxiliary resource data for LVMLogicalVolumeSpec.
type LVMLogicalVolumeSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMLogicalVolumeSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMLogicalVolumeSpecType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "VG", JSONPath: "{.vgName}"}, //nolint:goconst
			{Name: "Name", JSONPath: "{.name}"},
			{Name: "Type", JSONPath: "{.type}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMLogicalVolumeSpecType, &LVMLogicalVolumeSpec{}); err != nil {
		panic(err)
	}
}
