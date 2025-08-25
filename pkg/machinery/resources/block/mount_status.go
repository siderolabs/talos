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

// MountStatusType is type of MountStatus resource.
const MountStatusType = resource.Type("MountStatuses.block.talos.dev")

// MountStatus resource is a final mount request spec.
type MountStatus = typed.Resource[MountStatusSpec, MountStatusExtension]

// MountStatusSpec is the spec for MountStatus.
//
//gotagsrewrite:gen
type MountStatusSpec struct {
	Spec               MountRequestSpec       `yaml:"spec" protobuf:"1"`
	Source             string                 `yaml:"source" protobuf:"3"`
	Target             string                 `yaml:"target" protobuf:"2"`
	Filesystem         FilesystemType         `yaml:"filesystem" protobuf:"4"`
	EncryptionProvider EncryptionProviderType `yaml:"encryptionProvider,omitempty" protobuf:"7"`

	ReadOnly            bool `yaml:"readOnly" protobuf:"5"`
	ProjectQuotaSupport bool `yaml:"projectQuotaSupport" protobuf:"6"`
}

// NewMountStatus initializes a MountStatus resource.
func NewMountStatus(namespace resource.Namespace, id resource.ID) *MountStatus {
	return typed.NewResource[MountStatusSpec, MountStatusExtension](
		resource.NewMetadata(namespace, MountStatusType, id, resource.VersionUndefined),
		MountStatusSpec{},
	)
}

// MountStatusExtension is auxiliary resource data for BlockMountStatus.
type MountStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MountStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MountStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Source",
				JSONPath: `{.source}`,
			},
			{
				Name:     "Target",
				JSONPath: `{.target}`,
			},
			{
				Name:     "Filesystem",
				JSONPath: `{.filesystem}`,
			},
			{
				Name:     "Volume",
				JSONPath: `{.spec.volumeID}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(MountStatusType, &MountStatus{})
	if err != nil {
		panic(err)
	}
}
