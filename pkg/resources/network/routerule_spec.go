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

// RouteRuleSpecType is type of RouteRuleSpec resource.
const RouteRuleSpecType = resource.Type("RouteRuleSpecs.net.talos.dev")

// RouteRuleSpec resource holds route rule specification to be applied to the kernel.
type RouteRuleSpec struct {
	md   resource.Metadata
	spec RouteRuleSpecSpec
}

// RouteRuleSpecSpec describes the route rule.
type RouteRuleSpecSpec struct {
	Priority            int                     `yaml:"priority,omitempty"`
	Family              nethelpers.Family       `yaml:"family"`
	Table               nethelpers.RoutingTable `yaml:"table"`
	Mark                int                     `yaml:"mark"`
	Mask                int                     `yaml:"mask"`
	TypeOfService       int                     `yaml:"typeOfService"`
	TunnelID            int                     `yaml:"tunnelID"`
	Goto                int                     `yaml:"goto"`
	Destination         netaddr.IPPrefix        `yaml:"dst,omitempty"`
	Source              netaddr.IPPrefix        `yaml:"src,omitempty"`
	Flow                int                     `yaml:"flow"`
	InputInterfaceName  string                  `yaml:"inputInterfaceName"`
	OutputInterfaceName string                  `yaml:"outputInterfaceName"`
	Invert              bool                    `yaml:"invert"`
	DestinationPort     nethelpers.PortRange    `yaml:"destinationPort,omitempty"`
	SourcePort          nethelpers.PortRange    `yaml:"sourcePort,omitempty"`

	ConfigLayer ConfigLayer `yaml:"layer"`
}

// NewRouteRuleSpec initializes a RouteSpec resource.
func NewRouteRuleSpec(namespace resource.Namespace, id resource.ID) *RouteRuleSpec {
	r := &RouteRuleSpec{
		md:   resource.NewMetadata(namespace, RouteSpecType, id, resource.VersionUndefined),
		spec: RouteRuleSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *RouteRuleSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *RouteRuleSpec) Spec() interface{} {
	return r.spec
}

func (r *RouteRuleSpec) String() string {
	return fmt.Sprintf("network.RouteRuleSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *RouteRuleSpec) DeepCopy() resource.Resource {
	return &RouteRuleSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *RouteRuleSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RouteRuleSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *RouteRuleSpec) TypedSpec() *RouteRuleSpecSpec {
	return &r.spec
}
