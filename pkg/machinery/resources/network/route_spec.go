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
	"github.com/siderolabs/gen/value"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// RouteSpecType is type of RouteSpec resource.
const RouteSpecType = resource.Type("RouteSpecs.net.talos.dev")

// RouteSpec resource holds route specification to be applied to the kernel.
type RouteSpec = typed.Resource[RouteSpecSpec, RouteSpecExtension]

// RouteSpecSpec describes the route.
//
//gotagsrewrite:gen
type RouteSpecSpec struct {
	Family      nethelpers.Family        `yaml:"family" protobuf:"1"`
	Destination netip.Prefix             `yaml:"dst" protobuf:"2"`
	Source      netip.Addr               `yaml:"src" protobuf:"3"`
	Gateway     netip.Addr               `yaml:"gateway" protobuf:"4"`
	OutLinkName string                   `yaml:"outLinkName,omitempty" protobuf:"5"`
	Table       nethelpers.RoutingTable  `yaml:"table" protobuf:"6"`
	Priority    uint32                   `yaml:"priority,omitempty" protobuf:"7"`
	Scope       nethelpers.Scope         `yaml:"scope" protobuf:"8"`
	Type        nethelpers.RouteType     `yaml:"type" protobuf:"9"`
	Flags       nethelpers.RouteFlags    `yaml:"flags" protobuf:"10"`
	Protocol    nethelpers.RouteProtocol `yaml:"protocol" protobuf:"11"`
	ConfigLayer ConfigLayer              `yaml:"layer" protobuf:"12"`
	MTU         uint32                   `yaml:"mtu,omitempty" protobuf:"13"`
}

var (
	zero16 = netip.MustParseAddr("::")
	zero4  = netip.MustParseAddr("0.0.0.0")
)

// Normalize converts 0.0.0.0 to zero value.
//
//nolint:gocyclo
func (route *RouteSpecSpec) Normalize() nethelpers.Family {
	var family nethelpers.Family

	if route.Destination.Bits() == 0 {
		// clear destination to be zero value to support "0.0.0.0/0" routes
		if route.Destination.Addr().Compare(zero4) == 0 {
			family = nethelpers.FamilyInet4
			route.Destination = netip.Prefix{}
		}

		if route.Destination.Addr().Compare(zero16) == 0 {
			family = nethelpers.FamilyInet6
			route.Destination = netip.Prefix{}
		}
	}

	if route.Gateway.Compare(zero4) == 0 {
		family = nethelpers.FamilyInet4
		route.Gateway = netip.Addr{}
	}

	if route.Gateway.Compare(zero16) == 0 {
		family = nethelpers.FamilyInet6
		route.Gateway = netip.Addr{}
	}

	if route.Source.Compare(zero4) == 0 {
		family = nethelpers.FamilyInet4
		route.Source = netip.Addr{}
	}

	if route.Source.Compare(zero16) == 0 {
		family = nethelpers.FamilyInet6
		route.Source = netip.Addr{}
	}

	switch {
	case value.IsZero(route.Gateway) && !value.IsZero(route.Destination):
		route.Scope = nethelpers.ScopeLink
	case route.Destination.Addr().IsLoopback():
		route.Scope = nethelpers.ScopeHost
	default:
		route.Scope = nethelpers.ScopeGlobal
	}

	return family
}

// NewRouteSpec initializes a RouteSpec resource.
func NewRouteSpec(namespace resource.Namespace, id resource.ID) *RouteSpec {
	return typed.NewResource[RouteSpecSpec, RouteSpecExtension](
		resource.NewMetadata(namespace, RouteSpecType, id, resource.VersionUndefined),
		RouteSpecSpec{},
	)
}

// RouteSpecExtension provides auxiliary methods for RouteSpec.
type RouteSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RouteSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RouteSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[RouteSpecSpec](RouteSpecType, &RouteSpec{})
	if err != nil {
		panic(err)
	}
}
