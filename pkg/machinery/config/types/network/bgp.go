// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// BGPPeerKind is a BGP config document kind.
const BGPPeerKind = "BGPPeerConfig"

func init() {
	registry.Register(BGPPeerKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &BGPPeerConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkBGPPeerConfig = &BGPPeerConfigV1Alpha1{}
	_ config.Validator            = &BGPPeerConfigV1Alpha1{}
)

// BGPPeerConfigV1Alpha1 configures a native BGP speaker on the host.
//
//	examples:
//	  - value: exampleBGPPeerConfigV1Alpha1()
//	alias: BGPPeerConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/BGPPeerConfig
type BGPPeerConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Local autonomous system number for the BGP speaker.
	//   examples:
	//    - value: >
	//       uint32(65001)
	//   schemaRequired: true
	BGPLocalASN uint32 `yaml:"localASN"`
	//   description: |
	//     BGP router-id. If not set, it is derived from the first advertised address.
	//   examples:
	//    - value: >
	//       meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	//   schema:
	//     type: string
	BGPRouterID meta.Addr `yaml:"routerID,omitempty"`
	//   description: |
	//     Preferred source address set on routes installed from BGP (the kernel route `src` / RTA_PREFSRC,
	//     equivalent to FRR's `ip protocol bgp route-map SETSRC`). Set this to the node's loopback so that
	//     traffic following BGP-learned routes is sourced from the node identity even though the unnumbered
	//     fabric uplinks carry no address of their own. If not set, the kernel selects the source address.
	//   examples:
	//    - value: >
	//       meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	//   schema:
	//     type: string
	BGPRouteSource meta.Addr `yaml:"routeSource,omitempty"`
	//   description: |
	//     Names of the links whose addresses are originated into BGP as host routes (/32, /128).
	//     Typically a loopback or dummy link holding the node IP.
	//   examples:
	//    - value: >
	//       []string{"dummy0"}
	BGPAdvertise []string `yaml:"advertise,omitempty"`
	//   description: |
	//     Enable ECMP (multipath) for routes learned from multiple neighbors.
	BGPMultipath bool `yaml:"multipath,omitempty"`
	//   description: |
	//     Maximum number of ECMP next-hops to install. Zero uses the implementation default.
	BGPMaxPaths uint8 `yaml:"maxPaths,omitempty"`
	//   description: |
	//     BGP neighbors to peer with.
	BGPNeighborConfigs []BGPNeighborConfig `yaml:"neighbors,omitempty"`
}

// BGPNeighborConfig configures a single BGP neighbor.
type BGPNeighborConfig struct {
	//   description: |
	//     Neighbor IP address for a numbered session. Mutually exclusive with `link`.
	//   schema:
	//     type: string
	NeighborAddressConfig meta.Addr `yaml:"address,omitempty"`
	//   description: |
	//     Link name for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.
	//     Link aliases are supported.
	NeighborLinkConfig string `yaml:"link,omitempty"`
	//   description: |
	//     Expected peer ASN. Zero accepts any ASN advertised by the peer (eBGP "external").
	NeighborPeerASN uint32 `yaml:"peerASN,omitempty"`
	//   description: |
	//     BGP hold time. Zero uses the implementation default.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\.\d+|\d+)([nuµm]?s|m|h))|0)+$
	NeighborHoldTime time.Duration `yaml:"holdTime,omitempty"`
	//   description: |
	//     BFD (Bidirectional Forwarding Detection) configuration for the neighbor.
	NeighborBFDConfig *BGPBFDConfig `yaml:"bfd,omitempty"`
}

// BGPBFDConfig configures BFD for a BGP neighbor.
type BGPBFDConfig struct {
	//   description: |
	//     Desired minimum transmit interval. Zero uses the implementation default.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\.\d+|\d+)([nuµm]?s|m|h))|0)+$
	BFDTransmitInterval time.Duration `yaml:"transmitInterval,omitempty"`
	//   description: |
	//     Required minimum receive interval. Zero uses the implementation default.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\.\d+|\d+)([nuµm]?s|m|h))|0)+$
	BFDReceiveInterval time.Duration `yaml:"receiveInterval,omitempty"`
	//   description: |
	//     BFD detection multiplier. Zero uses the implementation default.
	BFDDetectMultiplier uint8 `yaml:"detectMultiplier,omitempty"`
}

// NewBGPPeerConfigV1Alpha1 creates a new BGPPeerConfig config document.
func NewBGPPeerConfigV1Alpha1() *BGPPeerConfigV1Alpha1 {
	return &BGPPeerConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       BGPPeerKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleBGPPeerConfigV1Alpha1() *BGPPeerConfigV1Alpha1 {
	cfg := NewBGPPeerConfigV1Alpha1()
	cfg.BGPLocalASN = 65001
	cfg.BGPAdvertise = []string{"dummy0"}
	cfg.BGPMultipath = true
	cfg.BGPNeighborConfigs = []BGPNeighborConfig{
		{
			NeighborLinkConfig: "enp1s0",
			NeighborBFDConfig:  &BGPBFDConfig{},
		},
		{
			NeighborLinkConfig: "enp2s0",
			NeighborBFDConfig:  &BGPBFDConfig{},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *BGPPeerConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// BGPPeerConfig implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) BGPPeerConfig() {}

// LocalASN implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) LocalASN() uint32 {
	return s.BGPLocalASN
}

// RouterID implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) RouterID() netip.Addr {
	return s.BGPRouterID.Addr
}

// RouteSource implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) RouteSource() netip.Addr {
	return s.BGPRouteSource.Addr
}

// AdvertiseLinks implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) AdvertiseLinks() []string {
	return s.BGPAdvertise
}

// Multipath implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) Multipath() bool {
	return s.BGPMultipath
}

// MaxPaths implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) MaxPaths() uint8 {
	return s.BGPMaxPaths
}

// Neighbors implements config.NetworkBGPPeerConfig interface.
func (s *BGPPeerConfigV1Alpha1) Neighbors() []config.NetworkBGPNeighbor {
	return xslices.Map(s.BGPNeighborConfigs, func(n BGPNeighborConfig) config.NetworkBGPNeighbor { return n })
}

// Validate implements config.Validator interface.
func (s *BGPPeerConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.BGPLocalASN == 0 {
		errs = errors.Join(errs, errors.New("localASN must be specified"))
	}

	if len(s.BGPNeighborConfigs) == 0 {
		warnings = append(warnings, "BGPPeerConfig has no neighbors configured")
	}

	for i, n := range s.BGPNeighborConfigs {
		hasAddress := n.NeighborAddressConfig.IsValid()
		hasLink := n.NeighborLinkConfig != ""

		if hasAddress == hasLink {
			errs = errors.Join(errs, fmt.Errorf("neighbor[%d]: exactly one of address or link must be set", i))
		}
	}

	return warnings, errs
}

// Address implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) Address() netip.Addr {
	return n.NeighborAddressConfig.Addr
}

// Link implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) Link() string {
	return n.NeighborLinkConfig
}

// PeerASN implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) PeerASN() uint32 {
	return n.NeighborPeerASN
}

// HoldTime implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) HoldTime() time.Duration {
	return n.NeighborHoldTime
}

// BFD implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) BFD() config.NetworkBGPBFD {
	if n.NeighborBFDConfig == nil {
		return nil
	}

	return n.NeighborBFDConfig
}

// TransmitInterval implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) TransmitInterval() time.Duration {
	return b.BFDTransmitInterval
}

// ReceiveInterval implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) ReceiveInterval() time.Duration {
	return b.BFDReceiveInterval
}

// DetectMultiplier implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) DetectMultiplier() uint8 {
	return b.BFDDetectMultiplier
}
