// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// ResolverSpecType is type of ResolverSpec resource.
const ResolverSpecType = resource.Type("ResolverSpecs.net.talos.dev")

// ResolverSpec resource holds DNS resolver info.
type ResolverSpec struct {
	md   resource.Metadata
	spec ResolverSpecSpec
}

// ResolverID is the ID of the singleton instance.
const ResolverID resource.ID = "resolvers"

// ResolverSpecSpec describes DNS resolvers.
type ResolverSpecSpec struct {
	DNSServers  []netaddr.IP `yaml:"dnsServers"`
	ConfigLayer ConfigLayer  `yaml:"layer"`
}

// NewResolverSpec initializes a ResolverSpec resource.
func NewResolverSpec(namespace resource.Namespace, id resource.ID) *ResolverSpec {
	r := &ResolverSpec{
		md:   resource.NewMetadata(namespace, ResolverSpecType, id, resource.VersionUndefined),
		spec: ResolverSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *ResolverSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *ResolverSpec) Spec() interface{} {
	return r.spec
}

func (r *ResolverSpec) String() string {
	return fmt.Sprintf("network.ResolverSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *ResolverSpec) DeepCopy() resource.Resource {
	return &ResolverSpec{
		md: r.md,
		spec: ResolverSpecSpec{
			DNSServers:  append([]netaddr.IP(nil), r.spec.DNSServers...),
			ConfigLayer: r.spec.ConfigLayer,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *ResolverSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ResolverSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *ResolverSpec) TypedSpec() *ResolverSpecSpec {
	return &r.spec
}
