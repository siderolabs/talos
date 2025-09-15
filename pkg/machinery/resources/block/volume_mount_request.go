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

// VolumeMountRequestType is type of VolumeMountRequest resource.
const VolumeMountRequestType = resource.Type("VolumeMountRequests.block.talos.dev")

// VolumeMountRequest resource holds a request of a subsystem to mount some volume.
type VolumeMountRequest = typed.Resource[VolumeMountRequestSpec, VolumeMountRequestExtension]

// VolumeMountRequestSpec is the spec for VolumeMountRequest.
//
//gotagsrewrite:gen
type VolumeMountRequestSpec struct {
	VolumeID string `yaml:"volumeID" protobuf:"1"`

	ReadOnly bool `yaml:"readOnly" protobuf:"3"`

	Detached bool `yaml:"detached" protobuf:"4"`

	Requester string `yaml:"requester" protobuf:"2"`
}

// NewVolumeMountRequest initializes a VolumeMountRequest resource.
func NewVolumeMountRequest(namespace resource.Namespace, id resource.ID) *VolumeMountRequest {
	return typed.NewResource[VolumeMountRequestSpec, VolumeMountRequestExtension](
		resource.NewMetadata(namespace, VolumeMountRequestType, id, resource.VersionUndefined),
		VolumeMountRequestSpec{},
	)
}

// VolumeMountRequestExtension is auxiliary resource data for BlockVolumeMountRequest.
type VolumeMountRequestExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeMountRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeMountRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume ID",
				JSONPath: `{.volumeID}`,
			},
			{
				Name:     "Requester",
				JSONPath: `{.requester}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(VolumeMountRequestType, &VolumeMountRequest{})
	if err != nil {
		panic(err)
	}
}
