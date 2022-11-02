// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ExtensionStatusType is type of Extension resource.
const ExtensionStatusType = resource.Type("ExtensionStatuses.runtime.talos.dev")

// ExtensionStatus resource holds status of installed system extensions.
type ExtensionStatus = typed.Resource[ExtensionStatusSpec, ExtensionStatusRD]

// ExtensionStatusSpec is the spec for system extensions.
type ExtensionStatusSpec = extensions.Layer

// NewExtensionStatus initializes a ExtensionStatus resource.
func NewExtensionStatus(namespace resource.Namespace, id resource.ID) *ExtensionStatus {
	return typed.NewResource[ExtensionStatusSpec, ExtensionStatusRD](
		resource.NewMetadata(namespace, ExtensionStatusType, id, resource.VersionUndefined),
		ExtensionStatusSpec{},
	)
}

// ExtensionStatusRD is auxiliary resource data for ExtensionStatus.
type ExtensionStatusRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ExtensionStatusRD) ResourceDefinition(resource.Metadata, ExtensionStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionStatusType,
		Aliases:          []resource.Type{"extensions"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: `{.metadata.name}`,
			},
			{
				Name:     "Version",
				JSONPath: `{.metadata.version}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ExtensionStatusSpec](ExtensionStatusType, &ExtensionStatus{})
	if err != nil {
		panic(err)
	}
}
