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

// VolumeMountStatusType is type of VolumeMountStatus resource.
const VolumeMountStatusType = resource.Type("VolumeMountStatuses.block.talos.dev")

// VolumeMountStatus resource holds a status of a subsystem to mount some volume.
type VolumeMountStatus = typed.Resource[VolumeMountStatusSpec, VolumeMountStatusExtension]

// VolumeMountStatusSpec is the spec for VolumeMountStatus.
//
//gotagsrewrite:gen
type VolumeMountStatusSpec struct {
	VolumeID  string `yaml:"volume_id" protobuf:"1"`
	Requester string `yaml:"requester" protobuf:"2"`

	Target   string `yaml:"target" protobuf:"3"`
	ReadOnly bool   `yaml:"read_only" protobuf:"4"`
}

// NewVolumeMountStatus initializes a VolumeMountStatus resource.
func NewVolumeMountStatus(namespace resource.Namespace, id resource.ID) *VolumeMountStatus {
	return typed.NewResource[VolumeMountStatusSpec, VolumeMountStatusExtension](
		resource.NewMetadata(namespace, VolumeMountStatusType, id, resource.VersionUndefined),
		VolumeMountStatusSpec{},
	)
}

// VolumeMountStatusExtension is auxiliary resource data for BlockVolumeMountStatus.
type VolumeMountStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeMountStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeMountStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume ID",
				JSONPath: `{.volume_id}`,
			},
			{
				Name:     "Requester",
				JSONPath: `{.requester}`,
			},
			{
				Name:     "Target",
				JSONPath: `{.target}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[VolumeMountStatusSpec](VolumeMountStatusType, &VolumeMountStatus{})
	if err != nil {
		panic(err)
	}
}
