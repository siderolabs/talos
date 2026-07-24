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
	BGPAdvertise []string `yaml:"advertise,omitempty" merge:"replace"`
	//   description: |
	//     Enable ECMP (multipath) for routes learned from multiple neighbors. Defaults to false.
	BGPMultipath *bool `yaml:"multipath,omitempty"`
	//   description: |
	//     Maximum number of ECMP next-hops to install. Zero uses the implementation default.
	BGPMaxPaths uint8 `yaml:"maxPaths,omitempty"`
	//   description: |
	//     Install routes learned from BGP neighbors into the Linux routing table. Defaults to true.
	//     When false, learned routes remain in this instance's BGP RIB and can still be selected by
	//     `importRoutes`, but Talos does not install them into the Linux FIB.
	BGPInstallRoutes *bool `yaml:"installRoutes,omitempty"`
	//   description: |
	//     Selected routes to import from other BGP instances. Imports are one-way: matching best paths
	//     learned from each source instance are preserved and advertised by this instance with its own
	//     next hop. Locally originated and previously imported paths are not recursively imported.
	//     Selectors in a single target instance must not overlap, giving each imported prefix one source.
	BGPImportRoutes []BGPImportRoute `yaml:"importRoutes,omitempty" merge:"replace"`
	//   description: |
	//     BGP neighbors in this routing instance.
	BGPNeighborConfigs []BGPNeighborConfig `yaml:"neighbors,omitempty" merge:"replace"`
}

// BGPImportRoute selects routes learned by another BGP instance for one-way import.
type BGPImportRoute struct {
	//   description: |
	//     Name of the source BGP instance.
	//   schemaRequired: true
	ImportBGPInstance string `yaml:"bgpInstance"`
	//   description: |
	//     CIDR selectors. A learned route matches when it is contained by one of these prefixes.
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+/\d{1,3}$
	//     minItems: 1
	//   schemaRequired: true
	ImportPrefixes []meta.Prefix `yaml:"prefixes"`
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
	cfg.BGPMultipath = new(true)
	cfg.BGPImportRoutes = []BGPImportRoute{{
		ImportBGPInstance: "workload",
		ImportPrefixes:    []meta.Prefix{{Prefix: netip.MustParsePrefix("198.51.100.0/24")}},
	}}
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
func (s *BGPInstanceConfigV1Alpha1) Multipath() bool {
	if s.BGPMultipath == nil {
		return false
	}

	return *s.BGPMultipath
}

// MaxPaths implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) MaxPaths() uint8 { return s.BGPMaxPaths }

// InstallRoutes implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) InstallRoutes() bool {
	if s.BGPInstallRoutes == nil {
		return true
	}

	return *s.BGPInstallRoutes
}

// ImportRoutes implements config.NetworkBGPInstanceConfig interface.
func (s *BGPInstanceConfigV1Alpha1) ImportRoutes() []config.NetworkBGPImportRoute {
	return xslices.Map(s.BGPImportRoutes, func(route BGPImportRoute) config.NetworkBGPImportRoute { return route })
}

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

	errs = errors.Join(errs, validateBGPImportRoutes(s.MetaName, s.BGPImportRoutes))

	return warnings, errs
}

//docgen:nodoc
type indexedBGPImportSelector struct {
	prefix     netip.Prefix
	routeIndex int
}

func validateBGPImportRoutes(instanceName string, routes []BGPImportRoute) error {
	var errs error

	seenSources := map[string]struct{}{}
	selectors := []indexedBGPImportSelector{}

	for i, route := range routes {
		errs = errors.Join(errs, validateBGPImportRoute(i, instanceName, route))

		if route.ImportBGPInstance != "" {
			if _, ok := seenSources[route.ImportBGPInstance]; ok {
				errs = errors.Join(errs, fmt.Errorf("importRoutes[%d]: duplicate source BGP instance %q", i, route.ImportBGPInstance))
			}

			seenSources[route.ImportBGPInstance] = struct{}{}
		}

		for _, configuredPrefix := range route.ImportPrefixes {
			prefix := configuredPrefix.Prefix
			if !prefix.IsValid() || prefix != prefix.Masked() {
				continue
			}

			for _, selector := range selectors {
				if selector.routeIndex != i && prefixesOverlap(selector.prefix, prefix) {
					errs = errors.Join(errs, fmt.Errorf(
						"importRoutes[%d]: prefix %s overlaps importRoutes[%d] prefix %s",
						i,
						prefix,
						selector.routeIndex,
						selector.prefix,
					))
				}
			}

			selectors = append(selectors, indexedBGPImportSelector{prefix: prefix, routeIndex: i})
		}
	}

	return errs
}

func validateBGPImportRoute(index int, instanceName string, route BGPImportRoute) error {
	var errs error

	switch route.ImportBGPInstance {
	case "":
		errs = errors.Join(errs, fmt.Errorf("importRoutes[%d]: bgpInstance must be specified", index))
	case instanceName:
		errs = errors.Join(errs, fmt.Errorf("importRoutes[%d]: cannot import from the same BGP instance %q", index, instanceName))
	}

	return errors.Join(errs, validateBGPImportPrefixes(index, route.ImportPrefixes))
}

func validateBGPImportPrefixes(index int, prefixes []meta.Prefix) error {
	var errs error

	if len(prefixes) == 0 {
		errs = errors.Join(errs, fmt.Errorf("importRoutes[%d]: at least one prefix must be specified", index))
	}

	for i, configuredPrefix := range prefixes {
		prefix := configuredPrefix.Prefix

		if !prefix.IsValid() {
			errs = errors.Join(errs, fmt.Errorf("importRoutes[%d].prefixes[%d]: invalid prefix", index, i))

			continue
		}

		if prefix != prefix.Masked() {
			errs = errors.Join(errs, fmt.Errorf("importRoutes[%d].prefixes[%d]: prefix %s must be masked", index, i, prefix))
		}

		for j := range i {
			other := prefixes[j].Prefix
			if !other.IsValid() || !prefixesOverlap(other, prefix) {
				continue
			}

			errs = errors.Join(errs, fmt.Errorf("importRoutes[%d].prefixes[%d]: prefix %s overlaps prefixes[%d] %s", index, i, prefix, j, other))
		}
	}

	return errs
}

func prefixesOverlap(a, b netip.Prefix) bool {
	return a.Addr().BitLen() == b.Addr().BitLen() && (a.Contains(b.Addr()) || b.Contains(a.Addr()))
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

// BGPInstance implements config.NetworkBGPImportRoute interface.
func (route BGPImportRoute) BGPInstance() string { return route.ImportBGPInstance }

// Prefixes implements config.NetworkBGPImportRoute interface.
func (route BGPImportRoute) Prefixes() []netip.Prefix {
	return xslices.Map(route.ImportPrefixes, func(prefix meta.Prefix) netip.Prefix { return prefix.Prefix })
}

// TransmitInterval implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) TransmitInterval() time.Duration { return b.BFDTransmitInterval }

// ReceiveInterval implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) ReceiveInterval() time.Duration { return b.BFDReceiveInterval }

// DetectMultiplier implements config.NetworkBGPBFD interface.
func (b *BGPBFDConfig) DetectMultiplier() uint8 { return b.BFDDetectMultiplier }
