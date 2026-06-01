// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage //nolint:dupl

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LVMPhysicalVolumeSpecType is the type of LVMPhysicalVolumeSpec resource.
const LVMPhysicalVolumeSpecType = resource.Type("LVMPhysicalVolumeSpecs.storage.talos.dev")

// LVMPhysicalVolumeSpec is the desired state for a physical volume backing
// a managed VG. One per disk matched by the selector.
type LVMPhysicalVolumeSpec = typed.Resource[LVMPhysicalVolumeSpecSpec, LVMPhysicalVolumeSpecExtension]

// LVMPhysicalVolumeSpecSpec is the spec for LVMPhysicalVolumeSpec resource.
//
//gotagsrewrite:gen
type LVMPhysicalVolumeSpecSpec struct {
	// Device is the block-device path to initialize as a PV.
	Device string `yaml:"device" protobuf:"1"`
	// VGName is the target volume group name.
	VGName string `yaml:"vgName" protobuf:"2"`
}

// NewLVMPhysicalVolumeSpec initializes a LVMPhysicalVolumeSpec resource.
func NewLVMPhysicalVolumeSpec(namespace resource.Namespace, id resource.ID) *LVMPhysicalVolumeSpec {
	return typed.NewResource[LVMPhysicalVolumeSpecSpec, LVMPhysicalVolumeSpecExtension](
		resource.NewMetadata(namespace, LVMPhysicalVolumeSpecType, id, resource.VersionUndefined),
		LVMPhysicalVolumeSpecSpec{},
	)
}

// LVMPhysicalVolumeSpecExtension is auxiliary resource data for LVMPhysicalVolumeSpec.
type LVMPhysicalVolumeSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMPhysicalVolumeSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMPhysicalVolumeSpecType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Device", JSONPath: "{.device}"},
			{Name: "VG", JSONPath: "{.vgName}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMPhysicalVolumeSpecType, &LVMPhysicalVolumeSpec{}); err != nil {
		panic(err)
	}
}
