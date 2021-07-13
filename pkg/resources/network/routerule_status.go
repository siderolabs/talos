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

// RouteRuleStatusType is type of RouteRuleStatus resource.
const RouteRuleStatusType = resource.Type("RouteRuleStatuses.net.talos.dev")

// RouteRuleStatus resource holds physical network link status.
type RouteRuleStatus struct {
	md   resource.Metadata
	spec RouteRuleStatusSpec
}

// RouteRuleStatusSpec describes status of rendered secrets.
type RouteRuleStatusSpec struct {
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
}

// NewRouteRuleStatus initializes a RouteRuleStatus resource.
func NewRouteRuleStatus(namespace resource.Namespace, id resource.ID) *RouteRuleStatus {
	r := &RouteRuleStatus{
		md:   resource.NewMetadata(namespace, RouteRuleStatusType, id, resource.VersionUndefined),
		spec: RouteRuleStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *RouteRuleStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *RouteRuleStatus) Spec() interface{} {
	return r.spec
}

func (r *RouteRuleStatus) String() string {
	return fmt.Sprintf("network.RouteRuleStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *RouteRuleStatus) DeepCopy() resource.Resource {
	return &RouteRuleStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *RouteRuleStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RouteRuleStatusType,
		Aliases:          []resource.Type{"route", "routes"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Priority",
				JSONPath: `{.priority}`,
			},
			{
				Name:     "Family",
				JSONPath: `{.family}`,
			},
			{
				Name:     "Mark",
				JSONPath: `{.mark}`,
			},
			{
				Name:     "Source",
				JSONPath: `{.source}`,
			},
			{
				Name:     "Destination",
				JSONPath: `{.destination}`,
			},
			{
				Name:     "Table",
				JSONPath: `{.table}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *RouteRuleStatus) TypedSpec() *RouteRuleStatusSpec {
	return &r.spec
}
