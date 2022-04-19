// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// RouteSpecType is type of RouteSpec resource.
const RouteSpecType = resource.Type("RouteSpecs.net.talos.dev")

// RouteSpec resource holds route specification to be applied to the kernel.
type RouteSpec = typed.Resource[RouteSpecSpec, RouteSpecRD]

// RouteSpecSpec describes the route.
type RouteSpecSpec struct {
	Family      nethelpers.Family        `yaml:"family"`
	Destination netaddr.IPPrefix         `yaml:"dst"`
	Source      netaddr.IP               `yaml:"src"`
	Gateway     netaddr.IP               `yaml:"gateway"`
	OutLinkName string                   `yaml:"outLinkName,omitempty"`
	Table       nethelpers.RoutingTable  `yaml:"table"`
	Priority    uint32                   `yaml:"priority,omitempty"`
	Scope       nethelpers.Scope         `yaml:"scope"`
	Type        nethelpers.RouteType     `yaml:"type"`
	Flags       nethelpers.RouteFlags    `yaml:"flags"`
	Protocol    nethelpers.RouteProtocol `yaml:"protocol"`
	ConfigLayer ConfigLayer              `yaml:"layer"`
}

// DeepCopy generates a deep copy of RouteSpecSpec.
func (route RouteSpecSpec) DeepCopy() RouteSpecSpec {
	return route
}

var (
	zero16 = netaddr.MustParseIP("::")
	zero4  = netaddr.MustParseIP("0.0.0.0")
)

// Normalize converts 0.0.0.0 to zero value.
func (route *RouteSpecSpec) Normalize() {
	if route.Destination.Bits() == 0 && (route.Destination.IP().Compare(zero4) == 0 || route.Destination.IP().Compare(zero16) == 0) {
		// clear destination to be zero value to support "0.0.0.0/0" routes
		route.Destination = netaddr.IPPrefix{}
	}

	if route.Gateway.Compare(zero4) == 0 || route.Gateway.Compare(zero16) == 0 {
		route.Gateway = netaddr.IP{}
	}

	if route.Source.Compare(zero4) == 0 || route.Source.Compare(zero16) == 0 {
		route.Source = netaddr.IP{}
	}

	switch {
	case route.Gateway.IsZero():
		route.Scope = nethelpers.ScopeLink
	case route.Destination.IP().IsLoopback():
		route.Scope = nethelpers.ScopeHost
	default:
		route.Scope = nethelpers.ScopeGlobal
	}
}

// NewRouteSpec initializes a RouteSpec resource.
func NewRouteSpec(namespace resource.Namespace, id resource.ID) *RouteSpec {
	return typed.NewResource[RouteSpecSpec, RouteSpecRD](
		resource.NewMetadata(namespace, RouteSpecType, id, resource.VersionUndefined),
		RouteSpecSpec{},
	)
}

// RouteSpecRD provides auxiliary methods for RouteSpec.
type RouteSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (RouteSpecRD) ResourceDefinition(resource.Metadata, RouteSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RouteSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
