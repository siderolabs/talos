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

// ResolverStatusType is type of ResolverStatus resource.
const ResolverStatusType = resource.Type("ResolverStatuses.net.talos.dev")

// ResolverStatus resource holds DNS resolver info.
type ResolverStatus struct {
	md   resource.Metadata
	spec ResolverStatusSpec
}

// ResolverStatusSpec describes DNS resolvers.
type ResolverStatusSpec struct {
	DNSServers []netaddr.IP `yaml:"dnsServers"`
}

// NewResolverStatus initializes a ResolverStatus resource.
func NewResolverStatus(namespace resource.Namespace, id resource.ID) *ResolverStatus {
	r := &ResolverStatus{
		md:   resource.NewMetadata(namespace, ResolverStatusType, id, resource.VersionUndefined),
		spec: ResolverStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *ResolverStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *ResolverStatus) Spec() interface{} {
	return r.spec
}

func (r *ResolverStatus) String() string {
	return fmt.Sprintf("network.ResolverStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *ResolverStatus) DeepCopy() resource.Resource {
	return &ResolverStatus{
		md: r.md,
		spec: ResolverStatusSpec{
			DNSServers: append([]netaddr.IP(nil), r.spec.DNSServers...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *ResolverStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ResolverStatusType,
		Aliases:          []resource.Type{"resolvers"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Resolvers",
				JSONPath: "{.dnsServers}",
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *ResolverStatus) TypedSpec() *ResolverStatusSpec {
	return &r.spec
}
