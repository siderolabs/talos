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

// LVMVolumeGroupSpecType is the type of LVMVolumeGroupSpec resource.
const LVMVolumeGroupSpecType = resource.Type("LVMVolumeGroupSpecs.storage.talos.dev")

// LVMVolumeGroupSpec is the desired state for a volume group.
type LVMVolumeGroupSpec = typed.Resource[LVMVolumeGroupSpecSpec, LVMVolumeGroupSpecExtension]

// LVMVolumeGroupSpecSpec is the spec for LVMVolumeGroupSpec resource.
//
//gotagsrewrite:gen
type LVMVolumeGroupSpecSpec struct {
	// Name is the volume group name.
	Name string `yaml:"name" protobuf:"1"`
	// PhysicalVolumes is the list of PV device paths.
	PhysicalVolumes []string `yaml:"physicalVolumes" protobuf:"2"`
}

// NewLVMVolumeGroupSpec initializes a LVMVolumeGroupSpec resource.
func NewLVMVolumeGroupSpec(namespace resource.Namespace, id resource.ID) *LVMVolumeGroupSpec {
	return typed.NewResource[LVMVolumeGroupSpecSpec, LVMVolumeGroupSpecExtension](
		resource.NewMetadata(namespace, LVMVolumeGroupSpecType, id, resource.VersionUndefined),
		LVMVolumeGroupSpecSpec{},
	)
}

// LVMVolumeGroupSpecExtension is auxiliary resource data for LVMVolumeGroupSpec.
type LVMVolumeGroupSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (LVMVolumeGroupSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LVMVolumeGroupSpecType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Name", JSONPath: "{.name}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(LVMVolumeGroupSpecType, &LVMVolumeGroupSpec{}); err != nil {
		panic(err)
	}
}
