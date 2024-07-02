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

// VolumeStatusType is type of VolumeStatus resource.
const VolumeStatusType = resource.Type("VolumeStatuses.block.talos.dev")

// VolumeStatus resource contains information about the volume status.
type VolumeStatus = typed.Resource[VolumeStatusSpec, VolumeStatusExtension]

// VolumeStatusSpec is the spec for VolumeStatus resource.
//
//gotagsrewrite:gen
type VolumeStatusSpec struct {
	Provisioned bool `yaml:"provisioned" protobuf:"1"`
	Located     bool `yaml:"located" protobuf:"2"`

	Location string `yaml:"location,omitempty" protobuf:"3"`
}

// NewVolumeStatus initializes a BlockVolumeStatus resource.
func NewVolumeStatus(namespace resource.Namespace, id resource.ID) *VolumeStatus {
	return typed.NewResource[VolumeStatusSpec, VolumeStatusExtension](
		resource.NewMetadata(namespace, VolumeStatusType, id, resource.VersionUndefined),
		VolumeStatusSpec{},
	)
}

// VolumeStatusExtension is auxiliary resource data for BlockVolumeStatus.
type VolumeStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Provisioned",
				JSONPath: `{.provisioned}`,
			},
			{
				Name:     "Located",
				JSONPath: `{.located}`,
			},
			{
				Name:     "Location",
				JSONPath: `{.location}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[VolumeStatusSpec](VolumeStatusType, &VolumeStatus{})
	if err != nil {
		panic(err)
	}
}
