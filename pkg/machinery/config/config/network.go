// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"net/netip"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// NetworkRuleConfig defines the interface to access network firewall configuration.
type NetworkRuleConfig interface {
	NetworkRuleConfigRules
	NetworkRuleConfigDefaultAction
}

// NetworkRuleConfigRules defines the interface to access network firewall configuration.
type NetworkRuleConfigRules interface {
	Rules() []NetworkRule
}

// NetworkRuleConfigDefaultAction defines the interface to access network firewall configuration.
type NetworkRuleConfigDefaultAction interface {
	DefaultAction() nethelpers.DefaultAction
}

// NetworkRuleConfigSignal is used to signal documents which implement either of the NetworkRuleConfig interfaces.
type NetworkRuleConfigSignal interface {
	NetworkRuleConfigSignal()
}

// NetworkRule defines a network firewall rule.
type NetworkRule interface {
	Protocol() nethelpers.Protocol
	PortRanges() [][2]uint16
	Subnets() []netip.Prefix
	ExceptSubnets() []netip.Prefix
}

// WrapNetworkRuleConfigList wraps a list of NetworkConfig into a single NetworkConfig aggregating the results.
func WrapNetworkRuleConfigList(configs ...NetworkRuleConfigSignal) NetworkRuleConfig {
	return networkRuleConfigWrapper(configs)
}

type networkRuleConfigWrapper []NetworkRuleConfigSignal

func (w networkRuleConfigWrapper) DefaultAction() nethelpers.DefaultAction {
	// DefaultAction zero value is 'accept' which is the default config value as well.
	return findFirstValue(
		filterDocuments[NetworkRuleConfigDefaultAction](w),
		func(c NetworkRuleConfigDefaultAction) nethelpers.DefaultAction {
			return c.DefaultAction()
		},
	)
}

func (w networkRuleConfigWrapper) Rules() []NetworkRule {
	return aggregateValues(
		filterDocuments[NetworkRuleConfigRules](w),
		func(c NetworkRuleConfigRules) []NetworkRule {
			return c.Rules()
		},
	)
}

// EthernetConfig defines a network interface configuration.
type EthernetConfig interface {
	NamedDocument
	Rings() EthernetRingsConfig
	Channels() EthernetChannelsConfig
	Features() map[string]bool
	WakeOnLAN() []nethelpers.WOLMode
}

// EthernetRingsConfig defines a configuration for Ethernet link rings.
type EthernetRingsConfig struct {
	RX           *uint32
	TX           *uint32
	RXMini       *uint32
	RXJumbo      *uint32
	RXBufLen     *uint32
	CQESize      *uint32
	TXPush       *bool
	RXPush       *bool
	TXPushBufLen *uint32
	TCPDataSplit *bool
}

// EthernetChannelsConfig defines a configuration for Ethernet link channels.
type EthernetChannelsConfig struct {
	RX       *uint32
	TX       *uint32
	Other    *uint32
	Combined *uint32
}

// NetworkStaticHostConfig defines a static host configuration.
type NetworkStaticHostConfig interface {
	IP() string
	Aliases() []string
}

// NetworkHostnameConfig defines a hostname configuration.
type NetworkHostnameConfig interface {
	Hostname() string
	AutoHostname() nethelpers.AutoHostnameKind
}

// NetworkPhysicalLinkConfig defines a physical network link configuration.
type NetworkPhysicalLinkConfig interface {
	PhysicalLinkConfig()
	NetworkCommonLinkConfig
}

// NetworkDummyLinkConfig defines a dummy network link configuration.
type NetworkDummyLinkConfig interface {
	DummyLinkConfig()
	NetworkHardwareAddressConfig
	NetworkCommonLinkConfig
}

// NetworkHardwareAddressConfig defines a hardware (MAC) address configuration.
type NetworkHardwareAddressConfig interface {
	HardwareAddress() optional.Optional[nethelpers.HardwareAddr]
}

// NetworkCommonLinkConfig defines common configuration for network links.
type NetworkCommonLinkConfig interface {
	NamedDocument
	Up() optional.Optional[bool]
	MTU() optional.Optional[uint32]
	Addresses() []NetworkAddressConfig
	Routes() []NetworkRouteConfig
}

// NetworkAddressConfig defines a network address configuration.
type NetworkAddressConfig interface {
	Address() netip.Prefix
	RoutePriority() optional.Optional[uint32]
}

// NetworkRouteConfig defines a network route configuration.
type NetworkRouteConfig interface {
	Destination() optional.Optional[netip.Prefix]
	Gateway() optional.Optional[netip.Addr]
	Source() optional.Optional[netip.Addr]
	MTU() optional.Optional[uint32]
	Metric() optional.Optional[uint32]
	Table() optional.Optional[nethelpers.RoutingTable]
}

// NetworkLinkAliasConfig defines a network link alias configuration.
type NetworkLinkAliasConfig interface {
	NamedDocument
	LinkSelector() cel.Expression
}

// NetworkDHCPConfig defines a DHCP configuration for a network link.
type NetworkDHCPConfig interface {
	NamedDocument
	NetworkDHCPConfig()
}

// NetworkDHCPv4Config defines a DHCPv4 configuration for a network link.
type NetworkDHCPv4Config interface {
	NamedDocument
	NetworkDHCPConfig
	NetworkDHCPv4Config() // signal method
	RouteMetric() optional.Optional[uint32]
	IgnoreHostname() optional.Optional[bool]
	ClientIdentifier() nethelpers.ClientIdentifier
	DUIDRaw() optional.Optional[nethelpers.HardwareAddr]
}

// NetworkDHCPv6Config defines a DHCPv6 configuration for a network link.
type NetworkDHCPv6Config interface {
	NamedDocument
	NetworkDHCPConfig
	NetworkDHCPv6Config() // signal method
	RouteMetric() optional.Optional[uint32]
	IgnoreHostname() optional.Optional[bool]
	ClientIdentifier() nethelpers.ClientIdentifier
	DUIDRaw() optional.Optional[nethelpers.HardwareAddr]
}

// NetworkVirtualIPConfig defines a common virtual IP configuration.
//
//nolint:iface
type NetworkVirtualIPConfig interface {
	NamedDocument
	Link() string
	VIP() netip.Addr
}

// NetworkLayer2VIPConfig defines a Layer 2 VIP configuration.
//
//nolint:iface
type NetworkLayer2VIPConfig interface {
	NetworkVirtualIPConfig
}

// NetworkHCloudVIPConfig defines a Hetzner Cloud VIP configuration.
type NetworkHCloudVIPConfig interface {
	NetworkVirtualIPConfig
	HCloudAPIToken() string
}

// NetworkVLANConfig defines a VLAN link configuration.
type NetworkVLANConfig interface {
	NamedDocument
	NetworkCommonLinkConfig
	VLANConfig()
	VLANID() uint16
	ParentLink() string
	VLANMode() optional.Optional[nethelpers.VLANProtocol]
}
