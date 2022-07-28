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

// ResolverStatusType is type of ResolverStatus resource.
const ResolverStatusType = resource.Type("ResolverStatuses.net.talos.dev")

// ResolverStatus resource holds DNS resolver info.
type ResolverStatus = typed.Resource[ResolverStatusSpec, ResolverStatusRD]

// ResolverStatusSpec describes DNS resolvers.
//gotagsrewrite:gen
type ResolverStatusSpec struct {
	DNSServers []netaddr.IP `yaml:"dnsServers" protobuf:"1"`
}

// NewResolverStatus initializes a ResolverStatus resource.
func NewResolverStatus(namespace resource.Namespace, id resource.ID) *ResolverStatus {
	return typed.NewResource[ResolverStatusSpec, ResolverStatusRD](
		resource.NewMetadata(namespace, ResolverStatusType, id, resource.VersionUndefined),
		ResolverStatusSpec{},
	)
}

// ResolverStatusRD provides auxiliary methods for ResolverStatus.
type ResolverStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (ResolverStatusRD) ResourceDefinition(resource.Metadata, ResolverStatusSpec) meta.ResourceDefinitionSpec {
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
