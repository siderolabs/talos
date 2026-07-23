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

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// BGPPeerConfigType is the type of the BGPPeerConfig resource.
const BGPPeerConfigType = resource.Type("BGPPeerConfigs.net.talos.dev")

// BGPPeerConfigID is the singleton BGPPeerConfig resource ID.
const BGPPeerConfigID resource.ID = "config"

// BGPPeerConfig contains the runtime configuration for the BGP speaker.
type BGPPeerConfig = typed.Resource[BGPPeerConfigSpec, BGPPeerConfigExtension]

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
}

// BGPPeerConfigSpec contains the complete runtime configuration for the BGP speaker.
//
//gotagsrewrite:gen
type BGPPeerConfigSpec struct {
	LocalASN       uint32                  `yaml:"localASN" protobuf:"1"`
	RouterID       netip.Addr              `yaml:"routerID,omitempty" protobuf:"2"`
	RouteSource    netip.Addr              `yaml:"routeSource,omitempty" protobuf:"3"`
	AdvertiseLinks []string                `yaml:"advertiseLinks,omitempty" protobuf:"4"`
	Multipath      bool                    `yaml:"multipath,omitempty" protobuf:"5"`
	MaxPaths       uint8                   `yaml:"maxPaths,omitempty" protobuf:"6"`
	Neighbors      []BGPNeighborConfigSpec `yaml:"neighbors,omitempty" protobuf:"7"`
}

// NewBGPPeerConfig initializes the singleton BGPPeerConfig resource.
func NewBGPPeerConfig() *BGPPeerConfig {
	return typed.NewResource[BGPPeerConfigSpec, BGPPeerConfigExtension](
		resource.NewMetadata(NamespaceName, BGPPeerConfigType, BGPPeerConfigID, resource.VersionUndefined),
		BGPPeerConfigSpec{},
	)
}

// BGPPeerConfigExtension provides auxiliary methods for BGPPeerConfig.
type BGPPeerConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BGPPeerConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BGPPeerConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Local ASN", JSONPath: "{.localASN}"},
			{Name: "Router ID", JSONPath: "{.routerID}"},
			{Name: "Neighbors", JSONPath: "{.neighbors}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[BGPPeerConfigSpec](BGPPeerConfigType, &BGPPeerConfig{}); err != nil {
		panic(err)
	}
}
