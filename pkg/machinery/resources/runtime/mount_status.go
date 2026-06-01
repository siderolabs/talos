// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MountStatusType is type of Mount resource.
const MountStatusType = resource.Type("MountStatuses.runtime.talos.dev")

// MountStatus resource holds defined sysctl flags status.
type MountStatus = typed.Resource[MountStatusSpec, MountStatusExtension]

// MountStatusSpec describes status of the defined sysctls.
//
//gotagsrewrite:gen
type MountStatusSpec struct {
	Source              string   `yaml:"source" protobuf:"1"`
	Target              string   `yaml:"target" protobuf:"2"`
	FilesystemType      string   `yaml:"filesystemType" protobuf:"3"`
	Options             []string `yaml:"options" protobuf:"4"`
	Encrypted           bool     `yaml:"encrypted" protobuf:"5"`
	EncryptionProviders []string `yaml:"encryptionProviders,omitempty" protobuf:"6"`
}

// NewMountStatus initializes a MountStatus resource.
func NewMountStatus(namespace resource.Namespace, id resource.ID) *MountStatus {
	return typed.NewResource[MountStatusSpec, MountStatusExtension](
		resource.NewMetadata(namespace, MountStatusType, id, resource.VersionUndefined),
		MountStatusSpec{},
	)
}

// MountStatusExtension is auxiliary resource data for MountStatus.
type MountStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MountStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:                 MountStatusType,
		Aliases:              []resource.Type{"mounts"},
		DefaultNamespace:     NamespaceName,
		SkipAutomaticAliases: true,
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
				Name:     "Filesystem Type",
				JSONPath: `{.filesystemType}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MountStatusSpec](MountStatusType, &MountStatus{})
	if err != nil {
		panic(err)
	}
}
