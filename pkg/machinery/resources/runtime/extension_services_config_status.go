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

// ExtensionServiceConfigStatusType is a type of ExtensionServiceConfig.
const ExtensionServiceConfigStatusType = resource.Type("ExtensionServiceConfigStatuses.runtime.talos.dev")

// ExtensionServiceConfigStatus represents a resource that describes status of rendered extensions service config files.
type ExtensionServiceConfigStatus = typed.Resource[ExtensionServiceConfigStatusSpec, ExtensionServiceConfigStatusExtension]

// ExtensionServiceConfigStatusSpec describes status of rendered extensions service config files.
//
//gotagsrewrite:gen
type ExtensionServiceConfigStatusSpec struct {
	SpecVersion string `yaml:"specVersion" protobuf:"1"`
}

// NewExtensionServiceConfigStatusSpec initializes a new ExtensionServiceConfigStatusSpec.
func NewExtensionServiceConfigStatusSpec(namespace resource.Namespace, id resource.ID) *ExtensionServiceConfigStatus {
	return typed.NewResource[ExtensionServiceConfigStatusSpec, ExtensionServiceConfigStatusExtension](
		resource.NewMetadata(namespace, ExtensionServiceConfigStatusType, id, resource.VersionUndefined),
		ExtensionServiceConfigStatusSpec{},
	)
}

// ExtensionServiceConfigStatusExtension provides auxiliary methods for ExtensionServiceConfig.
type ExtensionServiceConfigStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ExtensionServiceConfigStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionServiceConfigStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ExtensionServiceConfigStatusSpec](ExtensionServiceConfigStatusType, &ExtensionServiceConfigStatus{})
	if err != nil {
		panic(err)
	}
}
