// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// RouteStatusType is type of RouteStatus resource.
const RouteStatusType = resource.Type("RouteStatuses.net.talos.dev")

// RouteStatus resource holds physical network link status.
type RouteStatus struct {
	md   resource.Metadata
	spec RouteStatusSpec
}

// RouteStatusSpec describes status of rendered secrets.
type RouteStatusSpec struct {
	Family       nethelpers.Family        `yaml:"family"`
	Destination  netaddr.IPPrefix         `yaml:"dst"`
	Source       netaddr.IP               `yaml:"src"`
	Gateway      netaddr.IP               `yaml:"gateway"`
	OutLinkIndex uint32                   `yaml:"outLinkIndex,omitempty"`
	OutLinkName  string                   `yaml:"outLinkName,omitempty"`
	Table        nethelpers.RoutingTable  `yaml:"table"`
	Priority     uint32                   `yaml:"priority"`
	Scope        nethelpers.Scope         `yaml:"scope"`
	Type         nethelpers.RouteType     `yaml:"type"`
	Flags        nethelpers.RouteFlags    `yaml:"flags"`
	Protocol     nethelpers.RouteProtocol `yaml:"protocol"`
}

// NewRouteStatus initializes a RouteStatus resource.
func NewRouteStatus(namespace resource.Namespace, id resource.ID) *RouteStatus {
	r := &RouteStatus{
		md:   resource.NewMetadata(namespace, RouteStatusType, id, resource.VersionUndefined),
		spec: RouteStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *RouteStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *RouteStatus) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *RouteStatus) DeepCopy() resource.Resource {
	return &RouteStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *RouteStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RouteStatusType,
		Aliases:          []resource.Type{"route", "routes"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Destination",
				JSONPath: `{.dst}`,
			},
			{
				Name:     "Gateway",
				JSONPath: `{.gateway}`,
			},
			{
				Name:     "Link",
				JSONPath: `{.outLinkName}`,
			},
			{
				Name:     "Metric",
				JSONPath: `{.priority}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *RouteStatus) TypedSpec() *RouteStatusSpec {
	return &r.spec
}
