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

// ResolverStatusType is type of ResolverStatus resource.
const ResolverStatusType = resource.Type("ResolverStatuses.net.talos.dev")

// ResolverStatus resource holds DNS resolver info.
type ResolverStatus = typed.Resource[ResolverStatusSpec, ResolverStatusExtension]

// ResolverStatusSpec describes DNS resolvers.
//
//gotagsrewrite:gen
type ResolverStatusSpec struct {
	DNSServers    []netip.Addr `yaml:"dnsServers" protobuf:"1"`
	SearchDomains []string     `yaml:"searchDomains" protobuf:"2"`
}

// NewResolverStatus initializes a ResolverStatus resource.
func NewResolverStatus(namespace resource.Namespace, id resource.ID) *ResolverStatus {
	return typed.NewResource[ResolverStatusSpec, ResolverStatusExtension](
		resource.NewMetadata(namespace, ResolverStatusType, id, resource.VersionUndefined),
		ResolverStatusSpec{},
	)
}

// ResolverStatusExtension provides auxiliary methods for ResolverStatus.
type ResolverStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ResolverStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ResolverStatusType,
		Aliases:          []resource.Type{"resolvers"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
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

	err := protobuf.RegisterDynamic[ResolverStatusSpec](ResolverStatusType, &ResolverStatus{})
	if err != nil {
		panic(err)
	}
}
