// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LinkAliasSpecType is type of LinkAliasSpec resource.
const LinkAliasSpecType = resource.Type("LinkAliasSpecs.net.talos.dev")

// LinkAliasSpec resource tells which link should have which alias (name).
//
// If the link shouldn't have the alias, resource is removed.
type LinkAliasSpec = typed.Resource[LinkAliasSpecSpec, LinkAliasSpecExtension]

// LinkAliasSpecSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type LinkAliasSpecSpec struct {
	Alias string `yaml:"alias" protobuf:"1"`
}

// NewLinkAliasSpec initializes a LinkAliasSpec resource.
func NewLinkAliasSpec(namespace resource.Namespace, id resource.ID) *LinkAliasSpec {
	return typed.NewResource[LinkAliasSpecSpec, LinkAliasSpecExtension](
		resource.NewMetadata(namespace, LinkAliasSpecType, id, resource.VersionUndefined),
		LinkAliasSpecSpec{},
	)
}

// LinkAliasSpecExtension provides auxiliary methods for LinkAliasSpec.
type LinkAliasSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LinkAliasSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkAliasSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Alias",
				JSONPath: `{.alias}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LinkAliasSpecSpec](LinkAliasSpecType, &LinkAliasSpec{})
	if err != nil {
		panic(err)
	}
}
