// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// MountStatusType is type of Mount resource.
const MountStatusType = resource.Type("MountStatuses.runtime.talos.dev")

// MountStatus resource holds defined sysctl flags status.
type MountStatus = typed.Resource[MountStatusSpec, MountStatusRD]

// MountStatusSpec describes status of the defined sysctls.
//gotagsrewrite:gen
type MountStatusSpec struct {
	Source         string   `yaml:"source" protobuf:"1"`
	Target         string   `yaml:"target" protobuf:"2"`
	FilesystemType string   `yaml:"filesystemType" protobuf:"3"`
	Options        []string `yaml:"options" protobuf:"4"`
}

// NewMountStatus initializes a MountStatus resource.
func NewMountStatus(namespace resource.Namespace, id resource.ID) *MountStatus {
	return typed.NewResource[MountStatusSpec, MountStatusRD](
		resource.NewMetadata(namespace, MountStatusType, id, resource.VersionUndefined),
		MountStatusSpec{},
	)
}

// MountStatusRD is auxiliary resource data for MountStatus.
type MountStatusRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MountStatusRD) ResourceDefinition(resource.Metadata, MountStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MountStatusType,
		Aliases:          []resource.Type{"mounts"},
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
				Name:     "Filesystem Type",
				JSONPath: `{.filesystemType}`,
			},
		},
	}
}
