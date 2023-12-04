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

// ExtensionServicesConfigStatusType is a type of ExtensionServicesConfig.
const ExtensionServicesConfigStatusType = resource.Type("ExtensionServicesConfigStatuses.runtime.talos.dev")

// ExtensionServicesConfigStatus represents a resource that describes status of rendered extensions service config files.
type ExtensionServicesConfigStatus = typed.Resource[ExtensionServicesConfigStatusSpec, ExtensionServicesConfigStatusExtension]

// ExtensionServicesConfigStatusSpec describes status of rendered extensions service config files.
//
//gotagsrewrite:gen
type ExtensionServicesConfigStatusSpec struct {
	SpecVersion string `yaml:"specVersion" protobuf:"1"`
}

// NewExtensionServicesConfigStatusSpec initializes a new ExtensionServicesConfigStatusSpec.
func NewExtensionServicesConfigStatusSpec(namespace resource.Namespace, id resource.ID) *ExtensionServicesConfigStatus {
	return typed.NewResource[ExtensionServicesConfigStatusSpec, ExtensionServicesConfigStatusExtension](
		resource.NewMetadata(namespace, ExtensionServicesConfigStatusType, id, resource.VersionUndefined),
		ExtensionServicesConfigStatusSpec{},
	)
}

// ExtensionServicesConfigStatusExtension provides auxiliary methods for ExtensionServiceConfig.
type ExtensionServicesConfigStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ExtensionServicesConfigStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionServicesConfigStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ExtensionServicesConfigStatusSpec](ExtensionServicesConfigStatusType, &ExtensionServicesConfigStatus{})
	if err != nil {
		panic(err)
	}
}
