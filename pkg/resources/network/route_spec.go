// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// RouteSpecType is type of RouteSpec resource.
const RouteSpecType = resource.Type("RouteSpecs.net.talos.dev")

// RouteSpec resource holds route specification to be applied to the kernel.
type RouteSpec struct {
	md   resource.Metadata
	spec RouteSpecSpec
}

// RouteSpecSpec describes the route.
type RouteSpecSpec struct {
	Family      nethelpers.Family        `yaml:"family"`
	Destination netaddr.IPPrefix         `yaml:"dst"`
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

// NewRouteSpec initializes a SecretsStatus resource.
func NewRouteSpec(namespace resource.Namespace, id resource.ID) *RouteSpec {
	r := &RouteSpec{
		md:   resource.NewMetadata(namespace, RouteSpecType, id, resource.VersionUndefined),
		spec: RouteSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *RouteSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *RouteSpec) Spec() interface{} {
	return r.spec
}

func (r *RouteSpec) String() string {
	return fmt.Sprintf("network.RouteSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *RouteSpec) DeepCopy() resource.Resource {
	return &RouteSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *RouteSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RouteSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// Status sets pod status.
func (r *RouteSpec) Status() *RouteSpecSpec {
	return &r.spec
}
