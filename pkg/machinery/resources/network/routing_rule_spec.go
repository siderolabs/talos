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

// RoutingRuleSpecType is type of RoutingRuleSpec resource.
const RoutingRuleSpecType = resource.Type("RoutingRuleSpecs.net.talos.dev")

// RoutingRuleSpec resource holds routing rule specification to be applied to the kernel.
type RoutingRuleSpec = typed.Resource[RoutingRuleSpecSpec, RoutingRuleSpecExtension]

// RoutingRuleSpecSpec describes the routing rule.
//
//gotagsrewrite:gen
type RoutingRuleSpecSpec struct {
	Family      nethelpers.Family            `yaml:"family" protobuf:"1"`
	Src         netip.Prefix                 `yaml:"src" protobuf:"2"`
	Dst         netip.Prefix                 `yaml:"dst" protobuf:"3"`
	Table       nethelpers.RoutingTable      `yaml:"table" protobuf:"4"`
	Priority    uint32                       `yaml:"priority" protobuf:"5"`
	Action      nethelpers.RoutingRuleAction `yaml:"action" protobuf:"6"`
	IIFName     string                       `yaml:"iifName,omitempty" protobuf:"7"`
	OIFName     string                       `yaml:"oifName,omitempty" protobuf:"8"`
	FwMark      uint32                       `yaml:"fwMark,omitempty" protobuf:"9"`
	FwMask      uint32                       `yaml:"fwMask,omitempty" protobuf:"10"`
	ConfigLayer ConfigLayer                  `yaml:"layer" protobuf:"11"`
}

// NewRoutingRuleSpec initializes a RoutingRuleSpec resource.
func NewRoutingRuleSpec(namespace resource.Namespace, id resource.ID) *RoutingRuleSpec {
	return typed.NewResource[RoutingRuleSpecSpec, RoutingRuleSpecExtension](
		resource.NewMetadata(namespace, RoutingRuleSpecType, id, resource.VersionUndefined),
		RoutingRuleSpecSpec{},
	)
}

// RoutingRuleSpecExtension provides auxiliary methods for RoutingRuleSpec.
type RoutingRuleSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RoutingRuleSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RoutingRuleSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[RoutingRuleSpecSpec](RoutingRuleSpecType, &RoutingRuleSpec{})
	if err != nil {
		panic(err)
	}
}
