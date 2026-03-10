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

// RoutingRuleStatusType is type of RoutingRuleStatus resource.
const RoutingRuleStatusType = resource.Type("RoutingRuleStatuses.net.talos.dev")

// RoutingRuleStatus resource holds routing rule status observed from the kernel.
type RoutingRuleStatus = typed.Resource[RoutingRuleStatusSpec, RoutingRuleStatusExtension]

// RoutingRuleStatusSpec describes the observed routing rule state.
//
//gotagsrewrite:gen
type RoutingRuleStatusSpec struct {
	Family   nethelpers.Family            `yaml:"family" protobuf:"1"`
	Src      netip.Prefix                 `yaml:"src" protobuf:"2"`
	Dst      netip.Prefix                 `yaml:"dst" protobuf:"3"`
	Table    nethelpers.RoutingTable      `yaml:"table" protobuf:"4"`
	Priority uint32                       `yaml:"priority" protobuf:"5"`
	Action   nethelpers.RoutingRuleAction `yaml:"action" protobuf:"6"`
	IIFName  string                       `yaml:"iifName,omitempty" protobuf:"7"`
	OIFName  string                       `yaml:"oifName,omitempty" protobuf:"8"`
	FwMark   uint32                       `yaml:"fwMark,omitempty" protobuf:"9"`
	FwMask   uint32                       `yaml:"fwMask,omitempty" protobuf:"10"`
}

// NewRoutingRuleStatus initializes a RoutingRuleStatus resource.
func NewRoutingRuleStatus(namespace resource.Namespace, id resource.ID) *RoutingRuleStatus {
	return typed.NewResource[RoutingRuleStatusSpec, RoutingRuleStatusExtension](
		resource.NewMetadata(namespace, RoutingRuleStatusType, id, resource.VersionUndefined),
		RoutingRuleStatusSpec{},
	)
}

// RoutingRuleStatusExtension provides auxiliary methods for RoutingRuleStatus.
type RoutingRuleStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RoutingRuleStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RoutingRuleStatusType,
		Aliases:          []resource.Type{"routingrule", "routingrules"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Src",
				JSONPath: `{.src}`,
			},
			{
				Name:     "Dst",
				JSONPath: `{.dst}`,
			},
			{
				Name:     "Table",
				JSONPath: `{.table}`,
			},
			{
				Name:     "Priority",
				JSONPath: `{.priority}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[RoutingRuleStatusSpec](RoutingRuleStatusType, &RoutingRuleStatus{})
	if err != nil {
		panic(err)
	}
}
