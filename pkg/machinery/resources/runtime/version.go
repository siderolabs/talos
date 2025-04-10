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

// VersionType is type of VersionStatus resource.
const VersionType = resource.Type("Versions.runtime.talos.dev")

// Version resource holds version of Talos.
type Version = typed.Resource[VersionSpec, VersionExtension]

// VersionSpec describes version of Talos.
type VersionSpec struct {
	Version string `yaml:"version" protobuf:"1"`
}

// NewVersion initializes a VersionStatus resource.
func NewVersion() *Version {
	return typed.NewResource[VersionSpec, VersionExtension](
		resource.NewMetadata(NamespaceName, VersionType, "version", resource.VersionUndefined),
		VersionSpec{},
	)
}

// VersionExtension is auxiliary resource data for VersionStatus.
type VersionExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VersionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VersionType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Version",
				JSONPath: `{.version}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(VersionType, &Version{})
	if err != nil {
		panic(err)
	}
}
