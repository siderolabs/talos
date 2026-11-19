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

// InlineDataVolumeStatusType is type of InlineDataVolumeStatus resource.
const InlineDataVolumeStatusType = resource.Type("InlineDataVolumeStatuses.files.talos.dev")

// InlineDataVolumeStatus resource reports the files of an inline data volume as rendered.
type InlineDataVolumeStatus = typed.Resource[InlineDataVolumeStatusSpec, InlineDataVolumeStatusExtension]

// InlineDataVolumeStatusSpec describes the rendered files of an inline data volume.
//
//gotagsrewrite:gen
type InlineDataVolumeStatusSpec struct {
	// VolumeID is the ID of the volume holding the files.
	VolumeID string `yaml:"volumeID" protobuf:"1"`
	// ContentsHash is the hash of the rendered files.
	ContentsHash string `yaml:"contentsHash" protobuf:"2"`
	// Files is the number of rendered files.
	Files int `yaml:"files" protobuf:"3"`
	// Device is the device number of the filesystem holding the files.
	Device uint64 `yaml:"device" protobuf:"4"`
}

// NewInlineDataVolumeStatus initializes a InlineDataVolumeStatus resource.
func NewInlineDataVolumeStatus(namespace resource.Namespace, id resource.ID) *InlineDataVolumeStatus {
	return typed.NewResource[InlineDataVolumeStatusSpec, InlineDataVolumeStatusExtension](
		resource.NewMetadata(namespace, InlineDataVolumeStatusType, id, resource.VersionUndefined),
		InlineDataVolumeStatusSpec{},
	)
}

// InlineDataVolumeStatusExtension provides auxiliary methods for InlineDataVolumeStatus.
type InlineDataVolumeStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (InlineDataVolumeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             InlineDataVolumeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume",
				JSONPath: "{.volumeID}",
			},
			{
				Name:     "Files",
				JSONPath: "{.files}",
			},
			{
				Name:     "Hash",
				JSONPath: "{.contentsHash}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(InlineDataVolumeStatusType, &InlineDataVolumeStatus{})
	if err != nil {
		panic(err)
	}
}
