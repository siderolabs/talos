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

// ExtensionServiceConfigType is a type of ExtensionServiceConfig.
const ExtensionServiceConfigType = resource.Type("ExtensionServiceConfigs.runtime.talos.dev")

// ExtensionServiceConfig represents a resource that describes status of rendered extensions service config files.
type ExtensionServiceConfig = typed.Resource[ExtensionServiceConfigSpec, ExtensionServiceConfigExtension]

// ExtensionServiceConfigSpec describes status of rendered extensions service config files.
//
//gotagsrewrite:gen
type ExtensionServiceConfigSpec struct {
	Files       []ExtensionServiceConfigFile `yaml:"files,omitempty" protobuf:"1"`
	Environment []string                     `yaml:"environment,omitempty" protobuf:"2"`
}

// ExtensionServiceConfigFile describes extensions service config files.
//
//gotagsrewrite:gen
type ExtensionServiceConfigFile struct {
	Content   string `yaml:"content" protobuf:"1"`
	MountPath string `yaml:"mountPath" protobuf:"2"`
}

// NewExtensionServiceConfigSpec initializes a new ExtensionServiceConfigSpec.
func NewExtensionServiceConfigSpec(namespace resource.Namespace, id resource.ID) *ExtensionServiceConfig {
	return typed.NewResource[ExtensionServiceConfigSpec, ExtensionServiceConfigExtension](
		resource.NewMetadata(namespace, ExtensionServiceConfigType, id, resource.VersionUndefined),
		ExtensionServiceConfigSpec{},
	)
}

// ExtensionServiceConfigExtension provides auxiliary methods for ExtensionServiceConfig.
type ExtensionServiceConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ExtensionServiceConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionServiceConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ExtensionServiceConfigSpec](ExtensionServiceConfigType, &ExtensionServiceConfig{})
	if err != nil {
		panic(err)
	}
}
