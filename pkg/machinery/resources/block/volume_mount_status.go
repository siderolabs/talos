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
	"github.com/siderolabs/talos/pkg/machinery/resources"
)

// VolumeMountStatusType is type of VolumeMountStatus resource.
const VolumeMountStatusType = resource.Type("VolumeMountStatuses.block.talos.dev")

// VolumeMountStatus resource holds a status of a subsystem to mount some volume.
type VolumeMountStatus = typed.Resource[VolumeMountStatusSpec, VolumeMountStatusExtension]

// VolumeMountStatusSpec is the spec for VolumeMountStatus.
//
//gotagsrewrite:gen
type VolumeMountStatusSpec struct {
	VolumeID  string `yaml:"volumeID" protobuf:"1"`
	Requester string `yaml:"requester" protobuf:"2"`

	Target   string `yaml:"target" protobuf:"3"`
	ReadOnly bool   `yaml:"readOnly" protobuf:"4"`
	Detached bool   `yaml:"detached" protobuf:"5"`

	root any
}

// SetRoot sets the XFS root for the mount.
func (m *VolumeMountStatusSpec) SetRoot(root any) {
	m.root = root
}

// Root gets the XFS root for the mount.
// It's not guaranteed to be set (may be nil).
func (m *VolumeMountStatusSpec) Root() any {
	return m.root
}

// NewVolumeMountStatus initializes a VolumeMountStatus resource.
func NewVolumeMountStatus(id resource.ID) *VolumeMountStatus {
	return typed.NewResource[VolumeMountStatusSpec, VolumeMountStatusExtension](
		resource.NewMetadata(resources.InMemoryNamespace, VolumeMountStatusType, id, resource.VersionUndefined),
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
		DefaultNamespace: resources.InMemoryNamespace,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume ID",
				JSONPath: `{.volumeID}`,
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

	err := protobuf.RegisterDynamic(VolumeMountStatusType, &VolumeMountStatus{})
	if err != nil {
		panic(err)
	}
}
