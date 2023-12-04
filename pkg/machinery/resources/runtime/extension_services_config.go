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

// ExtensionServicesConfigType is a type of ExtensionServicesConfig.
const ExtensionServicesConfigType = resource.Type("ExtensionServicesConfigs.runtime.talos.dev")

// ExtensionServicesConfig represents a resource that describes status of rendered extensions service config files.
type ExtensionServicesConfig = typed.Resource[ExtensionServicesConfigSpec, ExtensionServicesConfigExtension]

// ExtensionServicesConfigSpec describes status of rendered extensions service config files.
//
//gotagsrewrite:gen
type ExtensionServicesConfigSpec struct {
	Files []ExtensionServicesConfigFile `yaml:"files" protobuf:"2"`
}

// ExtensionServicesConfigFile describes extensions service config files.
//
//gotagsrewrite:gen
type ExtensionServicesConfigFile struct {
	Content   string `yaml:"content" protobuf:"1"`
	MountPath string `yaml:"mountPath" protobuf:"2"`
}

// NewExtensionServicesConfigSpec initializes a new ExtensionServiceConfigSpec.
func NewExtensionServicesConfigSpec(namespace resource.Namespace, id resource.ID) *ExtensionServicesConfig {
	return typed.NewResource[ExtensionServicesConfigSpec, ExtensionServicesConfigExtension](
		resource.NewMetadata(namespace, ExtensionServicesConfigType, id, resource.VersionUndefined),
		ExtensionServicesConfigSpec{},
	)
}

// ExtensionServicesConfigExtension provides auxiliary methods for ExtensionServiceConfig.
type ExtensionServicesConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ExtensionServicesConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionServicesConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ExtensionServicesConfigSpec](ExtensionServicesConfigType, &ExtensionServicesConfig{})
	if err != nil {
		panic(err)
	}
}
