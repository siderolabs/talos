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

// MetaKeyType is type of MetaKey resource.
const MetaKeyType = resource.Type("MetaKeys.runtime.talos.dev")

// MetaKey resource holds value of a key in META partition.
type MetaKey = typed.Resource[MetaKeySpec, MetaKeyExtension]

// MetaKeySpec describes status of the defined sysctls.
//
//gotagsrewrite:gen
type MetaKeySpec struct {
	Value string `yaml:"value" protobuf:"1"`
}

// NewMetaKey initializes a MetaKey resource.
func NewMetaKey(namespace resource.Namespace, id resource.ID) *MetaKey {
	return typed.NewResource[MetaKeySpec, MetaKeyExtension](
		resource.NewMetadata(namespace, MetaKeyType, id, resource.VersionUndefined),
		MetaKeySpec{},
	)
}

// MetaKeyExtension is auxiliary resource data for MetaKey.
type MetaKeyExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MetaKeyExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MetaKeyType,
		Aliases:          []resource.Type{"meta"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Value",
				JSONPath: `{.value}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MetaKeySpec](MetaKeyType, &MetaKey{})
	if err != nil {
		panic(err)
	}
}
