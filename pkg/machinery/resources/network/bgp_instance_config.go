// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net/netip"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// BGPInstanceConfigType is the type of the BGPInstanceConfig resource.
const BGPInstanceConfigType = resource.Type("BGPInstanceConfigs.net.talos.dev")

// BGPInstanceConfig contains the resolved runtime configuration for a BGP routing instance.
type BGPInstanceConfig = typed.Resource[BGPInstanceConfigSpec, BGPInstanceConfigExtension]

// BGPBFDConfigSpec contains BFD parameters for a BGP neighbor.
//
//gotagsrewrite:gen
type BGPBFDConfigSpec struct {
	TransmitInterval time.Duration `yaml:"transmitInterval,omitempty" protobuf:"1"`
	ReceiveInterval  time.Duration `yaml:"receiveInterval,omitempty" protobuf:"2"`
	DetectMultiplier uint8         `yaml:"detectMultiplier,omitempty" protobuf:"3"`
}

// BGPNeighborConfigSpec contains the runtime configuration for a BGP neighbor.
//
//gotagsrewrite:gen
type BGPNeighborConfigSpec struct {
	Address  netip.Addr        `yaml:"address,omitempty" protobuf:"1"`
	Link     string            `yaml:"link,omitempty" protobuf:"2"`
	PeerASN  uint32            `yaml:"peerASN,omitempty" protobuf:"3"`
	HoldTime time.Duration     `yaml:"holdTime,omitempty" protobuf:"4"`
	BFD      *BGPBFDConfigSpec `yaml:"bfd,omitempty" protobuf:"5"`
	LocalASN uint32            `yaml:"localASN,omitempty" protobuf:"6"`
	Passive  bool              `yaml:"passive,omitempty" protobuf:"7"`
}

// BGPInstanceConfigSpec contains the resolved runtime configuration for a BGP routing instance.
//
//gotagsrewrite:gen
type BGPInstanceConfigSpec struct {
	LocalASN       uint32                  `yaml:"localASN" protobuf:"1"`
	RouterID       netip.Addr              `yaml:"routerID,omitempty" protobuf:"2"`
	RouteSource    netip.Addr              `yaml:"routeSource,omitempty" protobuf:"3"`
	AdvertiseLinks []string                `yaml:"advertiseLinks,omitempty" protobuf:"4"`
	Multipath      bool                    `yaml:"multipath,omitempty" protobuf:"5"`
	MaxPaths       uint8                   `yaml:"maxPaths,omitempty" protobuf:"6"`
	Neighbors      []BGPNeighborConfigSpec `yaml:"neighbors,omitempty" protobuf:"7"`
	VRF            string                  `yaml:"vrf,omitempty" protobuf:"8"`
	VRFTable       nethelpers.RoutingTable `yaml:"vrfTable,omitempty" protobuf:"9"`
}

// NewBGPInstanceConfig initializes a named BGPInstanceConfig resource.
func NewBGPInstanceConfig(id resource.ID) *BGPInstanceConfig {
	return typed.NewResource[BGPInstanceConfigSpec, BGPInstanceConfigExtension](
		resource.NewMetadata(NamespaceName, BGPInstanceConfigType, id, resource.VersionUndefined),
		BGPInstanceConfigSpec{},
	)
}

// BGPInstanceConfigExtension provides auxiliary methods for BGPInstanceConfig.
type BGPInstanceConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BGPInstanceConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BGPInstanceConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Local ASN", JSONPath: "{.localASN}"},
			{Name: "Router ID", JSONPath: "{.routerID}"},
			{Name: "VRF", JSONPath: "{.vrf}"},
			{Name: "Neighbors", JSONPath: "{.neighbors}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[BGPInstanceConfigSpec](BGPInstanceConfigType, &BGPInstanceConfig{}); err != nil {
		panic(err)
	}
}
