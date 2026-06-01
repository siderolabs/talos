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
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ResolverSpecType is type of ResolverSpec resource.
const ResolverSpecType = resource.Type("ResolverSpecs.net.talos.dev")

// ResolverSpec resource holds DNS resolver info.
type ResolverSpec = typed.Resource[ResolverSpecSpec, ResolverSpecExtension]

// ResolverID is the ID of the singleton instance.
const ResolverID resource.ID = "resolvers"

// NameServerSpec describes a single DNS nameserver with additional configuration.
//
//gotagsrewrite:gen
type NameServerSpec struct {
	Addr          netip.Addr             `yaml:"addr" protobuf:"1"`
	Protocol      nethelpers.DNSProtocol `yaml:"protocol" protobuf:"2"`
	TLSServerName string                 `yaml:"tlsServerName" protobuf:"3"`
}

// String returns a string representation of the NameServerSpec for logging purposes.
func (ns NameServerSpec) String() string {
	switch ns.Protocol {
	case nethelpers.DNSProtocolDNSOverTLS:
		return ns.Addr.String() + " (DoT, TLS Server Name: " + ns.TLSServerName + ")"
	case nethelpers.DNSProtocolDNSOverHTTP:
		return ns.Addr.String() + " (DoH, TLS Server Name: " + ns.TLSServerName + ")"
	case nethelpers.DNSProtocolDefault:
		return ns.Addr.String()
	default:
		return ns.Addr.String() + " (Unknown Protocol)"
	}
}

// ResolverSpecSpec describes DNS resolvers.
//
//gotagsrewrite:gen
type ResolverSpecSpec struct {
	// DNSServers is a flat list of DNS server IP addresses.
	//
	// Deprecated: This field is deprecated in favor of NameServers which contain more information.
	DNSServers []netip.Addr `yaml:"dnsServers" protobuf:"1"`
	// NameServers is a list of DNS servers with additional configuration.
	NameServers   []NameServerSpec `yaml:"nameServers,omitempty" protobuf:"4"`
	ConfigLayer   ConfigLayer      `yaml:"layer" protobuf:"2"`
	SearchDomains []string         `yaml:"searchDomains,omitempty" protobuf:"3"`
}

// NewResolverSpec initializes a ResolverSpec resource.
func NewResolverSpec(namespace resource.Namespace, id resource.ID) *ResolverSpec {
	return typed.NewResource[ResolverSpecSpec, ResolverSpecExtension](
		resource.NewMetadata(namespace, ResolverSpecType, id, resource.VersionUndefined),
		ResolverSpecSpec{},
	)
}

// Convert handles conversion of deprecated fields to the new ones and vice versa for backward compatibility.
func (s *ResolverSpecSpec) Convert() {
	if s.NameServers == nil && s.DNSServers != nil {
		s.NameServers = xslices.Map(s.DNSServers, func(addr netip.Addr) NameServerSpec {
			return NameServerSpec{
				Addr:     addr,
				Protocol: nethelpers.DNSProtocolDefault,
			}
		})
	} else if s.DNSServers == nil && s.NameServers != nil {
		s.DNSServers = xslices.Map(s.NameServers, func(ns NameServerSpec) netip.Addr {
			return ns.Addr
		})
	}
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
