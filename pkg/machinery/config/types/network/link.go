// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// LinkKind is a Link config document kind.
const LinkKind = "LinkConfig"

func init() {
	registry.Register(LinkKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &LinkConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

func conflictingLinkKinds(selfKind string) []string {
	return xslices.Filter([]string{
		BondKind,
		BridgeKind,
		DummyLinkKind,
		LinkKind,
		VLANKind,
		VRFKind,
		WireguardKind,
	}, func(kind string) bool {
		return kind != selfKind
	})
}

// Check interfaces.
var (
	_ config.NetworkPhysicalLinkConfig = &LinkConfigV1Alpha1{}
	_ config.ConflictingDocument       = &LinkConfigV1Alpha1{}
	_ config.NamedDocument             = &LinkConfigV1Alpha1{}
	_ config.Validator                 = &LinkConfigV1Alpha1{}
)

// LinkConfigV1Alpha1 is a config document to configure physical interfaces (network links).
//
//	examples:
//	  - value: exampleLinkConfigV1Alpha1()
//	alias: LinkConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/LinkConfig
type LinkConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the link (interface).
	//
	//   examples:
	//    - value: >
	//       "enp0s2"
	//    - value: >
	//       "eth1"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// CommonLinkConfig is common configuration for network links, and logical links.
type CommonLinkConfig struct {
	//   description: |
	//     Bring the link up or down.
	//
	//     If not specified, the link will be brought up.
	LinkUp *bool `yaml:"up,omitempty"`
	//   description: |
	//     Configure LinkMTU (Maximum Transmission Unit) for the link.
	//
	//     If not specified, the system default LinkMTU will be used (usually 1500).
	LinkMTU uint32 `yaml:"mtu,omitempty"`
	//   description: |
	//     Configure addresses to be statically assigned to the link.
	LinkAddresses []AddressConfig `yaml:"addresses,omitempty"`
	//   description: |
	//     Configure routes to be statically created via the link.
	LinkRoutes []RouteConfig `yaml:"routes,omitempty"`
	//   description: |
	//     Set the multicast capability of the link.
	LinkMulticast *bool `yaml:"multicast,omitempty"`
}

// AddressConfig represents a network address configuration.
type AddressConfig struct {
	//   description: |
	//     IP address to be assigned to the link.
	//
	//     This field must include the network prefix length (e.g. /24 for IPv4, /64 for IPv6).
	//   examples:
	//    - value: >
	//       netip.MustParsePrefix("192.168.1.100/24")
	//    - value: >
	//       netip.MustParsePrefix("fd00::1/64")
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f.:]+/\d{1,3}$
	//   schemaRequired: true
	AddressAddress netip.Prefix `yaml:"address"`
	//   description: |
	//     Configure the route priority (metric) for routes created for this address.
	//
	//     If not specified, the system default route priority will be used.
	AddressPriority *uint32 `yaml:"routePriority,omitempty"`
}

// RouteConfig represents a network route configuration.
type RouteConfig struct {
	//   description: |
	//    The route's destination as an address prefix.
	//
	//    If not specified, a default route will be created for the address family of the gateway.
	//   examples:
	//    - value: >
	//       Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f.:]+/\d{1,3}$
	RouteDestination Prefix `yaml:"destination,omitempty"`
	//   description: |
	//     The route's gateway (if empty, creates link scope route).
	//   examples:
	//    - value: >
	//       Addr{netip.MustParseAddr("10.0.0.1")}
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f.:]+$
	RouteGateway Addr `yaml:"gateway,omitempty"`
	//   description: |
	//     The route's source address (optional).
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f.:]+$
	RouteSource Addr `yaml:"source,omitempty"`
	//   description: |
	//     The optional metric for the route.
	RouteMetric uint32 `yaml:"metric,omitempty"`
	//   description: |
	//     The optional MTU for the route.
	RouteMTU uint32 `yaml:"mtu,omitempty"`
	//   description: |
	//     The routing table to use for the route.
	//
	//     If not specified, the main routing table will be used.
	//   schema:
	//     type: string
	RouteTable nethelpers.RoutingTable `yaml:"table,omitempty"`
}

// NewLinkConfigV1Alpha1 creates a new LinkConfig config document.
func NewLinkConfigV1Alpha1(name string) *LinkConfigV1Alpha1 {
	return &LinkConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       LinkKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleLinkConfigV1Alpha1() *LinkConfigV1Alpha1 {
	cfg := NewLinkConfigV1Alpha1("enp0s2")
	cfg.LinkMTU = 9000
	cfg.LinkUp = new(true)
	cfg.LinkAddresses = []AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("192.168.1.100/24"),
		},
		{
			AddressAddress: netip.MustParsePrefix("fd00::1/64"),
		},
	}
	cfg.LinkRoutes = []RouteConfig{
		{
			RouteDestination: Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			RouteGateway:     Addr{netip.MustParseAddr("10.0.0.1")},
		},
		{
			RouteGateway: Addr{netip.MustParseAddr("fe80::1")},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *LinkConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *LinkConfigV1Alpha1) Name() string {
	return s.MetaName
}

// PhysicalLinkConfig implements NetworkPhysicalLinkConfig interface.
func (s *LinkConfigV1Alpha1) PhysicalLinkConfig() {}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *LinkConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(LinkKind)
}

// Validate implements config.Validator interface.
func (s *LinkConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}

// Validate validates the common link config.
//
//nolint:gocyclo
func (s *CommonLinkConfig) Validate() ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	for i, addr := range s.LinkAddresses {
		switch {
		case addr.AddressAddress == (netip.Prefix{}):
			errs = errors.Join(errs, fmt.Errorf("address %d must be specified", i))
		case !addr.AddressAddress.IsValid():
			errs = errors.Join(errs, fmt.Errorf("address %d must be a valid IP prefix", i))
		case !addr.AddressAddress.Addr().IsValid() || addr.AddressAddress.Addr().IsUnspecified():
			errs = errors.Join(errs, fmt.Errorf("address %d must be a valid IP address", i))
		}
	}

	for i, route := range s.LinkRoutes {
		if route.RouteDestination != (Prefix{}) && (!route.RouteDestination.IsValid() || route.RouteDestination.Addr().IsUnspecified()) {
			errs = errors.Join(errs, fmt.Errorf("route %d destination must be a valid IP prefix", i))
		}

		if route.RouteGateway != (Addr{}) && (!route.RouteGateway.IsValid() || route.RouteGateway.IsUnspecified()) {
			errs = errors.Join(errs, fmt.Errorf("route %d gateway must be a valid IP address", i))
		}

		if route.RouteSource != (Addr{}) && (!route.RouteSource.IsValid() || route.RouteSource.IsUnspecified()) {
			errs = errors.Join(errs, fmt.Errorf("route %d source must be a valid IP address", i))
		}
	}

	return warnings, errs
}

// Up implements NetworkCommonLinkConfig interface.
func (s *CommonLinkConfig) Up() optional.Optional[bool] {
	if s.LinkUp == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.LinkUp)
}

// MTU implements NetworkCommonLinkConfig interface.
func (s *CommonLinkConfig) MTU() optional.Optional[uint32] {
	if s.LinkMTU == 0 {
		return optional.None[uint32]()
	}

	return optional.Some(s.LinkMTU)
}

// Addresses implements NetworkCommonLinkConfig interface.
func (s *CommonLinkConfig) Addresses() []config.NetworkAddressConfig {
	return xslices.Map(s.LinkAddresses, func(a AddressConfig) config.NetworkAddressConfig {
		return a
	})
}

// Routes implements NetworkCommonLinkConfig interface.
func (s *CommonLinkConfig) Routes() []config.NetworkRouteConfig {
	return xslices.Map(s.LinkRoutes, func(r RouteConfig) config.NetworkRouteConfig {
		return r
	})
}

// Multicast implements NetworkCommonLinkConfig interface.
func (s *CommonLinkConfig) Multicast() optional.Optional[bool] {
	if s.LinkMulticast == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.LinkMulticast)
}

// Address implements NetworkAddressConfig interface.
func (a AddressConfig) Address() netip.Prefix {
	return a.AddressAddress
}

// RoutePriority implements NetworkAddressConfig interface.
func (a AddressConfig) RoutePriority() optional.Optional[uint32] {
	if a.AddressPriority == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*a.AddressPriority)
}

// Destination implements NetworkRouteConfig interface.
func (r RouteConfig) Destination() optional.Optional[netip.Prefix] {
	if r.RouteDestination == (Prefix{}) {
		return optional.None[netip.Prefix]()
	}

	return optional.Some(r.RouteDestination.Prefix)
}

// Gateway implements NetworkRouteConfig interface.
func (r RouteConfig) Gateway() optional.Optional[netip.Addr] {
	if r.RouteGateway == (Addr{}) {
		return optional.None[netip.Addr]()
	}

	return optional.Some(r.RouteGateway.Addr)
}

// Source implements NetworkRouteConfig interface.
func (r RouteConfig) Source() optional.Optional[netip.Addr] {
	if r.RouteSource == (Addr{}) {
		return optional.None[netip.Addr]()
	}

	return optional.Some(r.RouteSource.Addr)
}

// MTU implements NetworkRouteConfig interface.
func (r RouteConfig) MTU() optional.Optional[uint32] {
	if r.RouteMTU == 0 {
		return optional.None[uint32]()
	}

	return optional.Some(r.RouteMTU)
}

// Metric implements NetworkRouteConfig interface.
func (r RouteConfig) Metric() optional.Optional[uint32] {
	if r.RouteMetric == 0 {
		return optional.None[uint32]()
	}

	return optional.Some(r.RouteMetric)
}

// Table implements NetworkRouteConfig interface.
func (r RouteConfig) Table() optional.Optional[nethelpers.RoutingTable] {
	if r.RouteTable == nethelpers.TableUnspec {
		return optional.None[nethelpers.RoutingTable]()
	}

	return optional.Some(r.RouteTable)
}
