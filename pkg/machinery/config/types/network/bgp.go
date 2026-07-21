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

const (
	// BGPInstanceKind is a BGP instance config document kind.
	BGPInstanceKind = "BGPInstanceConfig"
)

func init() {
	registry.Register(BGPInstanceKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &BGPInstanceConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkBGPInstanceConfig = &BGPInstanceConfigV1Alpha1{}
	_ config.NamedDocument            = &BGPInstanceConfigV1Alpha1{}
	_ config.Validator                = &BGPInstanceConfigV1Alpha1{}
)

// BGPInstanceConfigV1Alpha1 configures a native BGP routing instance on the host.
//
//	examples:
//	  - value: exampleBGPInstanceConfigV1Alpha1()
//	alias: BGPInstanceConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/BGPInstanceConfig
type BGPInstanceConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the BGP routing instance.
	//   examples:
	//    - value: >
	//       "fabric"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Linux VRF link used by this routing instance. If unset, the default routing domain is used.
	BGPVRF string `yaml:"vrf,omitempty"`
	//   description: |
	//     Local autonomous system number for the BGP instance.
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
	//     equivalent to FRR's `ip protocol bgp route-map SETSRC`). If not set, the kernel selects the source address.
	//   examples:
	//    - value: >
	//       meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	//   schema:
	//     type: string
	BGPRouteSource meta.Addr `yaml:"routeSource,omitempty"`
	//   description: |
	//     Names or aliases of the links whose addresses are originated into BGP as host routes (/32, /128).
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
	//     BGP neighbors in this routing instance.
	BGPNeighborConfigs []BGPNeighborConfig `yaml:"neighbors,omitempty"`
}

// BGPNeighborConfig configures a concrete BGP neighbor.
type BGPNeighborConfig struct {
	//   description: |
	//     Neighbor IP address for a numbered session. Mutually exclusive with `link`.
	//   schema:
	//     type: string
	NeighborAddressConfig meta.Addr `yaml:"address,omitempty"`
	//   description: |
	//     Link name or alias for an unnumbered (IPv6 link-local) session. Mutually exclusive with `address`.
	NeighborLinkConfig string `yaml:"link,omitempty"`
	//   description: |
	//     Expected peer ASN. Zero accepts any ASN advertised by the peer (eBGP "external").
	NeighborPeerASN uint32 `yaml:"peerASN,omitempty"`
	//   description: |
	//     Local ASN override for this neighbor. Zero uses the instance local ASN.
	NeighborLocalASN uint32 `yaml:"localASN,omitempty"`
	//   description: |
	//     Wait for the neighbor to establish the connection instead of initiating it.
	NeighborPassive bool `yaml:"passive,omitempty"`
	//   description: |
	//     BGP hold time for this neighbor. Zero uses the implementation default.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\.\d+|\d+)([nuµm]?s|m|h))|0)+$
	NeighborHoldTime time.Duration `yaml:"holdTime,omitempty"`
	//   description: |
	//     BFD (Bidirectional Forwarding Detection) settings for this neighbor.
	//     The presence of this block enables BFD; an empty block uses the implementation defaults.
	//     BFD is supported only when the BGP instance uses the default routing domain, not a VRF.
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

// NewBGPInstanceConfigV1Alpha1 creates a new BGPInstanceConfig config document.
func NewBGPInstanceConfigV1Alpha1(name string) *BGPInstanceConfigV1Alpha1 {
	return &BGPInstanceConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       BGPInstanceKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleBGPInstanceConfigV1Alpha1() *BGPInstanceConfigV1Alpha1 {
	cfg := NewBGPInstanceConfigV1Alpha1("fabric")
	cfg.BGPLocalASN = 65001
	cfg.BGPAdvertise = []string{"dummy0"}
	cfg.BGPMultipath = true
	cfg.BGPNeighborConfigs = []BGPNeighborConfig{
		{
			NeighborLinkConfig: "enp1s0",
			NeighborPeerASN:    65000,
			NeighborHoldTime:   9 * time.Second,
			NeighborBFDConfig: &BGPBFDConfig{
				BFDTransmitInterval: 300 * time.Millisecond,
				BFDReceiveInterval:  300 * time.Millisecond,
				BFDDetectMultiplier: 3,
			},
		},
		{
			NeighborLinkConfig: "enp2s0",
			NeighborPeerASN:    65000,
			NeighborHoldTime:   9 * time.Second,
			NeighborBFDConfig: &BGPBFDConfig{
				BFDTransmitInterval: 300 * time.Millisecond,
				BFDReceiveInterval:  300 * time.Millisecond,
				BFDDetectMultiplier: 3,
			},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *BGPInstanceConfigV1Alpha1) Clone() config.Document { return s.DeepCopy() }

// Name implements config.NamedDocument interface.
func (s *BGPInstanceConfigV1Alpha1) Name() string { return s.MetaName }

// BGPInstanceConfigSignal implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) BGPInstanceConfigSignal() {}

// VRF implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) VRF() string { return s.BGPVRF }

// LocalASN implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) LocalASN() uint32 { return s.BGPLocalASN }

// RouterID implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) RouterID() netip.Addr { return s.BGPRouterID.Addr }

// RouteSource implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) RouteSource() netip.Addr { return s.BGPRouteSource.Addr }

// AdvertiseLinks implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) AdvertiseLinks() []string { return s.BGPAdvertise }

// Multipath implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) Multipath() bool { return s.BGPMultipath }

// MaxPaths implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) MaxPaths() uint8 { return s.BGPMaxPaths }

// Neighbors implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) Neighbors() []config.NetworkBGPNeighbor {
	return xslices.Map(s.BGPNeighborConfigs, func(n BGPNeighborConfig) config.NetworkBGPNeighbor { return n })
}

// Validate implements config.Validator interface.
func (s *BGPInstanceConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.BGPLocalASN == 0 {
		errs = errors.Join(errs, errors.New("localASN must be specified"))
	}

	if s.BGPRouterID.IsValid() && !s.BGPRouterID.Addr.Is4() {
		errs = errors.Join(errs, errors.New("routerID must be an IPv4 address"))
	}

	if len(s.BGPNeighborConfigs) == 0 {
		warnings = append(warnings, "BGPInstanceConfig has no neighbors configured")
	}

	seen := map[string]struct{}{}

	for i, n := range s.BGPNeighborConfigs {
		key, err := bgpNeighborKey(i, n)
		errs = errors.Join(errs, err, validateBGPNeighborBehavior(i, n, s.BGPVRF))

		if err != nil {
			continue
		}

		if _, ok := seen[key]; ok {
			errs = errors.Join(errs, fmt.Errorf("neighbor[%d]: duplicate neighbor %q", i, key))
		}

		seen[key] = struct{}{}
	}

	return warnings, errs
}

func bgpNeighborKey(index int, neighbor BGPNeighborConfig) (string, error) {
	hasAddress := neighbor.NeighborAddressConfig.IsValid()
	hasLink := neighbor.NeighborLinkConfig != ""

	if hasAddress == hasLink {
		return "", fmt.Errorf("neighbor[%d]: exactly one of address or link must be set", index)
	}

	if hasAddress {
		return neighbor.NeighborAddressConfig.String(), nil
	}

	return neighbor.NeighborLinkConfig, nil
}

func validateBGPNeighborBehavior(index int, neighbor BGPNeighborConfig, vrf string) error {
	var errs error

	if neighbor.NeighborHoldTime < 0 {
		errs = errors.Join(errs, fmt.Errorf("neighbor[%d]: holdTime must not be negative", index))
	}

	if neighbor.NeighborBFDConfig == nil {
		return errs
	}

	if vrf != "" {
		errs = errors.Join(errs, fmt.Errorf("neighbor[%d]: bfd is not supported for VRF-bound BGP instances", index))
	}

	if neighbor.NeighborBFDConfig.BFDTransmitInterval < 0 {
		errs = errors.Join(errs, fmt.Errorf("neighbor[%d]: bfd.transmitInterval must not be negative", index))
	}

	if neighbor.NeighborBFDConfig.BFDReceiveInterval < 0 {
		errs = errors.Join(errs, fmt.Errorf("neighbor[%d]: bfd.receiveInterval must not be negative", index))
	}

	return errs
}

// Address implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) Address() netip.Addr { return n.NeighborAddressConfig.Addr }

// Link implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) Link() string { return n.NeighborLinkConfig }

// PeerASN implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) PeerASN() uint32 { return n.NeighborPeerASN }

// LocalASN implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) LocalASN() uint32 { return n.NeighborLocalASN }

// Passive implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) Passive() bool { return n.NeighborPassive }

// HoldTime implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) HoldTime() time.Duration { return n.NeighborHoldTime }

// BFD implements config.NetworkBGPNeighbor interface.
func (n BGPNeighborConfig) BFD() config.NetworkBGPBFD {
	if n.NeighborBFDConfig == nil {
		return nil
	}

	return n.NeighborBFDConfig
}

// TransmitInterval implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) TransmitInterval() time.Duration { return b.BFDTransmitInterval }

// ReceiveInterval implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) ReceiveInterval() time.Duration { return b.BFDReceiveInterval }

// DetectMultiplier implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) DetectMultiplier() uint8 { return b.BFDDetectMultiplier }
