// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ResolverSpecType is type of ResolverSpec resource.
const ResolverSpecType = resource.Type("ResolverSpecs.net.talos.dev")

// ResolverSpec resource holds DNS resolver info.
type ResolverSpec = typed.Resource[ResolverSpecSpec, ResolverSpecExtension]

// ResolverID is the ID of the singleton instance.
const ResolverID resource.ID = "resolvers"

// ResolverSpecSpec describes DNS resolvers.
//
//gotagsrewrite:gen
type ResolverSpecSpec struct {
	DNSServers    []netip.Addr `yaml:"dnsServers" protobuf:"1"`
	ConfigLayer   ConfigLayer  `yaml:"layer" protobuf:"2"`
	SearchDomains []string     `yaml:"searchDomains,omitempty" protobuf:"3"`
}

// NewResolverSpec initializes a ResolverSpec resource.
func NewResolverSpec(namespace resource.Namespace, id resource.ID) *ResolverSpec {
	return typed.NewResource[ResolverSpecSpec, ResolverSpecExtension](
		resource.NewMetadata(namespace, ResolverSpecType, id, resource.VersionUndefined),
		ResolverSpecSpec{},
	)
}

// ResolverSpecExtension provides auxiliary methods for ResolverSpec.
type ResolverSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ResolverSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ResolverSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Layer",
				JSONPath: "{.layer}",
			},
			{
				Name:     "Resolvers",
				JSONPath: "{.dnsServers}",
			},
			{
				Name:     "Search Domains",
				JSONPath: "{.searchDomains}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ResolverSpecSpec](ResolverSpecType, &ResolverSpec{})
	if err != nil {
		panic(err)
	}
}
