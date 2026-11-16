// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ExtensionStatusType is type of Extension resource.
const ExtensionStatusType = resource.Type("ExtensionStatuses.runtime.talos.dev")

// ExtensionStatus resource holds status of installed system extensions.
type ExtensionStatus = typed.Resource[ExtensionStatusSpec, ExtensionStatusExtension]

// ExtensionStatusSpec is the spec for system extensions.
type ExtensionStatusSpec = extensions.Layer

// NewExtensionStatus initializes a ExtensionStatus resource.
func NewExtensionStatus(namespace resource.Namespace, id resource.ID) *ExtensionStatus {
	return typed.NewResource[ExtensionStatusSpec, ExtensionStatusExtension](
		resource.NewMetadata(namespace, ExtensionStatusType, id, resource.VersionUndefined),
		ExtensionStatusSpec{},
	)
}

// ExtensionInstalled reports whether the extension with the given name is among the extension statuses.
//
// The resource ID is derived from the layer image, so the match is on the name the extension declares.
func ExtensionInstalled(extensions safe.List[*ExtensionStatus], name string) bool {
	_, found := extensions.Find(func(ext *ExtensionStatus) bool {
		return ext.TypedSpec().Metadata.Name == name
	})

	return found
}

// ExtensionStatusExtension is auxiliary resource data for ExtensionStatus.
type ExtensionStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ExtensionStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
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
