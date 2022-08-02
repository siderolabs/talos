// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"inet.af/netaddr"
)

// ResolverSpecType is type of ResolverSpec resource.
const ResolverSpecType = resource.Type("ResolverSpecs.net.talos.dev")

// ResolverSpec resource holds DNS resolver info.
type ResolverSpec = typed.Resource[ResolverSpecSpec, ResolverSpecRD]

// ResolverID is the ID of the singleton instance.
const ResolverID resource.ID = "resolvers"

// ResolverSpecSpec describes DNS resolvers.
//
//gotagsrewrite:gen
type ResolverSpecSpec struct {
	DNSServers  []netaddr.IP `yaml:"dnsServers" protobuf:"1"`
	ConfigLayer ConfigLayer  `yaml:"layer" protobuf:"2"`
}

// NewResolverSpec initializes a ResolverSpec resource.
func NewResolverSpec(namespace resource.Namespace, id resource.ID) *ResolverSpec {
	return typed.NewResource[ResolverSpecSpec, ResolverSpecRD](
		resource.NewMetadata(namespace, ResolverSpecType, id, resource.VersionUndefined),
		ResolverSpecSpec{},
	)
}

// ResolverSpecRD provides auxiliary methods for ResolverSpec.
type ResolverSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (ResolverSpecRD) ResourceDefinition(resource.Metadata, ResolverSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ResolverSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
