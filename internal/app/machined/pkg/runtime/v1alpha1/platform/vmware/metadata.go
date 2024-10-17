// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vmware provides the VMware platform implementation.
package vmware

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NetworkConfig maps to VMware GuestInfo metadata.
// See also definition of GuestInfo in CAPV https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/pkg/util/constants.go
type NetworkConfig struct {
	InstanceID    string `yaml:"instance-id"`
	LocalHostname string `yaml:"local-hostname"`
	// Talos doesn't block on network, it will reconfigure itself as network information becomes available. WaitOnNetwork is not used.
	WaitOnNetwork struct {
		Ipv4 bool `yaml:"ipv4"`
		Ipv6 bool `yaml:"ipv6"`
	} `yaml:"wait-on-network,omitempty"`
	Network struct {
		Version   int                 `yaml:"version"`
		Ethernets map[string]Ethernet `yaml:"ethernets"`
	}
	Routes []Route `yaml:"routes,omitempty"`
}

// Ethernet holds network interface info.
type Ethernet struct {
	Match struct {
		Name   string `yaml:"name,omitempty"`
		HWAddr string `yaml:"macaddress,omitempty"`
	} `yaml:"match,omitempty"`
	SetName        string        `yaml:"set-name,omitempty"`
	Wakeonlan      bool          `yaml:"wakeonlan,omitempty"`
	DHCPv4         bool          `yaml:"dhcp4,omitempty"`
	DHCP4Overrides DHCPOverrides `yaml:"dhcp4-overrides,omitempty"`
	DHCPv6         bool          `yaml:"dhcp6,omitempty"`
	DHCP6Overrides DHCPOverrides `yaml:"dhcp6-overrides,omitempty"`
	Address        []string      `yaml:"addresses,omitempty"`
	Gateway4       string        `yaml:"gateway4,omitempty"`
	Gateway6       string        `yaml:"gateway6,omitempty"`
	MTU            int           `yaml:"mtu,omitempty"`
	NameServers    struct {
		Search  []string `yaml:"search,omitempty"`
		Address []string `yaml:"addresses,omitempty"`
	} `yaml:"nameservers,omitempty"`
	Routes []Route `yaml:"routes,omitempty"`
}

// Route configuration. Not used.
type Route struct {
	To     string `yaml:"to,omitempty"`
	Via    string `yaml:"via,omitempty"`
	Metric string `yaml:"metric,omitempty"`
}

// DHCPOverrides is partial implemented. Only RouteMetric is use, the other elements are not processed.
type DHCPOverrides struct {
	Hostname     string `yaml:"hostname,omitempty"`
	RouteMetric  uint32 `yaml:"route-metric,omitempty"`
	SendHostname string `yaml:"send-hostname,omitempty"`
	UseDNS       string `yaml:"use-dns,omitempty"`
	UseDomains   string `yaml:"use-domains,omitempty"`
	UseHostname  string `yaml:"use-hostname,omitempty"`
	UseMTU       string `yaml:"use-mtu,omitempty"`
	UseNTP       string `yaml:"use-ntp,omitempty"`
	UseRoutes    string `yaml:"use-routes,omitempty"`
}

// ApplyNetworkConfigV2 gets GuestInfo and applies to the Talos runtime platform network configuration.
//
//nolint:gocyclo,cyclop
func (v *VMware) ApplyNetworkConfigV2(ctx context.Context, st state.State, config *NetworkConfig, networkConfig *runtime.PlatformNetworkConfig) error {
	var dnsIPs []netip.Addr

	hostInterfaces, err := safe.StateListAll[*network.LinkStatus](ctx, st)
	if err != nil {
		return fmt.Errorf("error listing host interfaces: %w", err)
	}

	for name, eth := range config.Network.Ethernets {
		if eth.SetName != "" {
			name = eth.SetName
		}

		if !strings.HasPrefix(name, "eth") {
			continue
		}

		if eth.Match.HWAddr != "" {
			var availableMACAddresses []string

			macAddressMatched := false

			for hostInterface := range hostInterfaces.All() {
				macAddress := hostInterface.TypedSpec().PermanentAddr.String()
				if macAddress == eth.Match.HWAddr {
					name = hostInterface.Metadata().ID()
					macAddressMatched = true

					break
				}

				availableMACAddresses = append(availableMACAddresses, macAddress)
			}

			if !macAddressMatched {
				log.Printf("vmware: no link with matching MAC address %q (available %v), defaulted to use name %s instead", eth.Match.HWAddr, availableMACAddresses, name)
			}
		}

		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        name,
			Up:          true,
			MTU:         uint32(eth.MTU),
			ConfigLayer: network.ConfigPlatform,
		})

		if eth.DHCPv4 {
			routeMetric := uint32(network.DefaultRouteMetric)

			if eth.DHCP4Overrides.RouteMetric != 0 {
				routeMetric = eth.DHCP4Overrides.RouteMetric
			}

			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  name,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: routeMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}

		if eth.DHCPv6 {
			routeMetric := uint32(2 * network.DefaultRouteMetric)

			if eth.DHCP4Overrides.RouteMetric != 0 {
				routeMetric = eth.DHCP6Overrides.RouteMetric
			}

			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  name,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: routeMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}

		for _, addr := range eth.Address {
			ipPrefix, err := netip.ParsePrefix(addr)
			if err != nil {
				return err
			}

			family := nethelpers.FamilyInet4

			if ipPrefix.Addr().Is6() {
				family = nethelpers.FamilyInet6
			}

			networkConfig.Addresses = append(networkConfig.Addresses,
				network.AddressSpecSpec{
					ConfigLayer: network.ConfigPlatform,
					LinkName:    name,
					Address:     ipPrefix,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					Family:      family,
				},
			)
		}

		if eth.Gateway4 != "" {
			gw, err := netip.ParseAddr(eth.Gateway4)
			if err != nil {
				return err
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Gateway:     gw,
				OutLinkName: name,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      nethelpers.FamilyInet4,
				Priority:    network.DefaultRouteMetric,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}

		if eth.Gateway6 != "" {
			gw, err := netip.ParseAddr(eth.Gateway6)
			if err != nil {
				return err
			}

			route := network.RouteSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Gateway:     gw,
				OutLinkName: name,
				Table:       nethelpers.TableMain,
				Protocol:    nethelpers.ProtocolStatic,
				Type:        nethelpers.TypeUnicast,
				Family:      nethelpers.FamilyInet6,
				Priority:    2 * network.DefaultRouteMetric,
			}

			route.Normalize()

			networkConfig.Routes = append(networkConfig.Routes, route)
		}

		for _, addr := range eth.NameServers.Address {
			if ip, err := netip.ParseAddr(addr); err == nil {
				dnsIPs = append(dnsIPs, ip)
			} else {
				return err
			}
		}
	}

	if config.LocalHostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(config.LocalHostname); err != nil {
			return err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	if len(dnsIPs) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	return nil
}
