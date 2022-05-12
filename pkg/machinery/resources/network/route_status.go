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

// RouteStatusType is type of RouteStatus resource.
const RouteStatusType = resource.Type("RouteStatuses.net.talos.dev")

// RouteStatus resource holds physical network link status.
type RouteStatus = typed.Resource[RouteStatusSpec, RouteStatusRD]

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
	return typed.NewResource[RouteStatusSpec, RouteStatusRD](
		resource.NewMetadata(namespace, RouteStatusType, id, resource.VersionUndefined),
		RouteStatusSpec{},
	)
}

// RouteStatusRD provides auxiliary methods for RouteStatus.
type RouteStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (RouteStatusRD) ResourceDefinition(resource.Metadata, RouteStatusSpec) meta.ResourceDefinitionSpec {
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
