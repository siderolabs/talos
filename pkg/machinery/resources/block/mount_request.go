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

// MountRequestType is type of MountRequest resource.
const MountRequestType = resource.Type("MountRequests.block.talos.dev")

// MountRequest resource is a final mount request spec.
type MountRequest = typed.Resource[MountRequestSpec, MountRequestExtension]

// MountRequestSpec is the spec for MountRequest.
//
//gotagsrewrite:gen
type MountRequestSpec struct {
	VolumeID string `yaml:"volumeID" protobuf:"1"`

	ParentMountID string `yaml:"parentID" protobuf:"2"`
	ReadOnly      bool   `yaml:"readOnly" protobuf:"5"`
	Detached      bool   `yaml:"detached" protobuf:"6"`

	DisableAccessTime bool `yaml:"disableAccessTime,omitempty" protobuf:"7"`
	Secure            bool `yaml:"secure,omitempty" protobuf:"8"`

	Requesters   []string `yaml:"requesters" protobuf:"3"`
	RequesterIDs []string `yaml:"requesterIDs" protobuf:"4"`
}

// NewMountRequest initializes a MountRequest resource.
func NewMountRequest(namespace resource.Namespace, id resource.ID) *MountRequest {
	return typed.NewResource[MountRequestSpec, MountRequestExtension](
		resource.NewMetadata(namespace, MountRequestType, id, resource.VersionUndefined),
		MountRequestSpec{},
	)
}

// MountRequestExtension is auxiliary resource data for BlockMountRequest.
type MountRequestExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MountRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MountRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume",
				JSONPath: `{.volumeID}`,
			},
			{
				Name:     "Parent",
				JSONPath: `{.parentID}`,
			},
			{
				Name:     "Requesters",
				JSONPath: `{.requesters}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(MountRequestType, &MountRequest{})
	if err != nil {
		panic(err)
	}
}
