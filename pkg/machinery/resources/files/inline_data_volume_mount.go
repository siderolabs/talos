// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// InlineDataVolumeMountRequestType is type of InlineDataVolumeMountRequest resource.
const InlineDataVolumeMountRequestType = resource.Type("InlineDataVolumeMountRequests.files.talos.dev")

// InlineDataVolumeMountRequest resource asks for a read-only bind of an inline data volume for a consumer.
type InlineDataVolumeMountRequest = typed.Resource[InlineDataVolumeMountRequestSpec, InlineDataVolumeMountRequestExtension]

// InlineDataVolumeMountRequestSpec describes a read-only bind of an inline data volume.
//
//gotagsrewrite:gen
type InlineDataVolumeMountRequestSpec struct {
	// VolumeName is the name of the InlineDataVolumeConfig document.
	VolumeName string `yaml:"volumeName" protobuf:"1"`
	// Requester is the consumer of the bind.
	Requester string `yaml:"requester" protobuf:"2"`
}

// NewInlineDataVolumeMountRequest initializes an InlineDataVolumeMountRequest resource.
func NewInlineDataVolumeMountRequest(namespace resource.Namespace, id resource.ID) *InlineDataVolumeMountRequest {
	return typed.NewResource[InlineDataVolumeMountRequestSpec, InlineDataVolumeMountRequestExtension](
		resource.NewMetadata(namespace, InlineDataVolumeMountRequestType, id, resource.VersionUndefined),
		InlineDataVolumeMountRequestSpec{},
	)
}

// InlineDataVolumeMountRequestExtension provides auxiliary methods for InlineDataVolumeMountRequest.
type InlineDataVolumeMountRequestExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InlineDataVolumeMountRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InlineDataVolumeMountRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume",
				JSONPath: "{.volumeName}",
			},
			{
				Name:     "Requester",
				JSONPath: "{.requester}",
			},
		},
	}
}

// InlineDataVolumeMountStatusType is type of InlineDataVolumeMountStatus resource.
const InlineDataVolumeMountStatusType = resource.Type("InlineDataVolumeMountStatuses.files.talos.dev")

// InlineDataVolumeMountStatus resource reports a read-only bind of an inline data volume as ready.
type InlineDataVolumeMountStatus = typed.Resource[InlineDataVolumeMountStatusSpec, InlineDataVolumeMountStatusExtension]

// InlineDataVolumeMountStatusSpec describes a read-only bind of an inline data volume.
//
//gotagsrewrite:gen
type InlineDataVolumeMountStatusSpec struct {
	// VolumeName is the name of the InlineDataVolumeConfig document.
	VolumeName string `yaml:"volumeName" protobuf:"1"`
	// Target is the host path of the read-only bind.
	Target string `yaml:"target" protobuf:"2"`
	// ContentsHash is the hash of the most recently rendered files.
	ContentsHash string `yaml:"contentsHash" protobuf:"3"`
	// Device is the device number of the bound filesystem.
	Device uint64 `yaml:"device" protobuf:"4"`
}

// NewInlineDataVolumeMountStatus initializes an InlineDataVolumeMountStatus resource.
func NewInlineDataVolumeMountStatus(namespace resource.Namespace, id resource.ID) *InlineDataVolumeMountStatus {
	return typed.NewResource[InlineDataVolumeMountStatusSpec, InlineDataVolumeMountStatusExtension](
		resource.NewMetadata(namespace, InlineDataVolumeMountStatusType, id, resource.VersionUndefined),
		InlineDataVolumeMountStatusSpec{},
	)
}

// InlineDataVolumeMountStatusExtension provides auxiliary methods for InlineDataVolumeMountStatus.
type InlineDataVolumeMountStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InlineDataVolumeMountStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InlineDataVolumeMountStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume",
				JSONPath: "{.volumeName}",
			},
			{
				Name:     "Target",
				JSONPath: "{.target}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(InlineDataVolumeMountRequestType, &InlineDataVolumeMountRequest{}); err != nil {
		panic(err)
	}

	if err := protobuf.RegisterDynamic(InlineDataVolumeMountStatusType, &InlineDataVolumeMountStatus{}); err != nil {
		panic(err)
	}
}
