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

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// RouteStatusType is type of RouteStatus resource.
const RouteStatusType = resource.Type("RouteStatuses.net.talos.dev")

// RouteStatus resource holds physical network link status.
type RouteStatus = typed.Resource[RouteStatusSpec, RouteStatusExtension]

// RouteStatusSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type RouteStatusSpec struct {
	Family       nethelpers.Family        `yaml:"family" protobuf:"1"`
	Destination  netip.Prefix             `yaml:"dst" protobuf:"2"`
	Source       netip.Addr               `yaml:"src" protobuf:"3"`
	Gateway      netip.Addr               `yaml:"gateway" protobuf:"4"`
	OutLinkIndex uint32                   `yaml:"outLinkIndex,omitempty" protobuf:"5"`
	OutLinkName  string                   `yaml:"outLinkName,omitempty" protobuf:"6"`
	Table        nethelpers.RoutingTable  `yaml:"table" protobuf:"7"`
	Priority     uint32                   `yaml:"priority" protobuf:"8"`
	Scope        nethelpers.Scope         `yaml:"scope" protobuf:"9"`
	Type         nethelpers.RouteType     `yaml:"type" protobuf:"10"`
	Flags        nethelpers.RouteFlags    `yaml:"flags" protobuf:"11"`
	Protocol     nethelpers.RouteProtocol `yaml:"protocol" protobuf:"12"`
	MTU          uint32                   `yaml:"mtu,omitempty" protobuf:"13"`
}

// NewRouteStatus initializes a RouteStatus resource.
func NewRouteStatus(namespace resource.Namespace, id resource.ID) *RouteStatus {
	return typed.NewResource[RouteStatusSpec, RouteStatusExtension](
		resource.NewMetadata(namespace, RouteStatusType, id, resource.VersionUndefined),
		RouteStatusSpec{},
	)
}

// RouteStatusExtension provides auxiliary methods for RouteStatus.
type RouteStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RouteStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
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
			{
				Name:     "Table",
				JSONPath: `{.table}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[RouteStatusSpec](RouteStatusType, &RouteStatus{})
	if err != nil {
		panic(err)
	}
}
