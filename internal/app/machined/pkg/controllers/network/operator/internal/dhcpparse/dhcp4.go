// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dhcpparse converts DHCP packets into network configuration specs.
//
// It is split out of the operator package so the parsing logic can be
// unit-tested through a public surface, without reaching into operator
// internals.
package dhcpparse

import (
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/siderolabs/gen/xslices"
	"go4.org/netipx"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DHCP4AckSpecs holds the network configuration parsed out of a DHCPv4 ACK.
//
// All slices are nil when the ACK carries no data of that kind, so assigning
// the whole struct fully replaces previous lease state.
type DHCP4AckSpecs struct {
	Addresses   []network.AddressSpecSpec
	Links       []network.LinkSpecSpec
	Routes      []network.RouteSpecSpec
	Hostname    []network.HostnameSpecSpec
	Resolvers   []network.ResolverSpecSpec
	TimeServers []network.TimeServerSpecSpec
}

// ParseDHCP4Ack converts a DHCPv4 ACK packet into network configuration specs.
//
// It is a pure function (no I/O, no shared state) — `linkName` and
// `routeMetric` come from the operator's configuration, and `useHostname`
// controls whether the ACK's hostname/domain options are honored.
//
//nolint:gocyclo
func ParseDHCP4Ack(ack *dhcpv4.DHCPv4, linkName string, routeMetric uint32, useHostname bool) DHCP4AckSpecs {
	var specs DHCP4AckSpecs

	addr, _ := netipx.FromStdIPNet(&net.IPNet{
		IP:   ack.YourIPAddr,
		Mask: ack.SubnetMask(),
	})

	specs.Addresses = []network.AddressSpecSpec{
		{
			Address:     addr,
			LinkName:    linkName,
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			Priority:    routeMetric,
			ConfigLayer: network.ConfigOperator,
		},
	}

	if mtu, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, ack.Options); err == nil {
		specs.Links = []network.LinkSpecSpec{
			{
				Name: linkName,
				MTU:  uint32(mtu),
				Up:   true,
			},
		}
	}

	// rfc3442:
	//   If the DHCP server returns both a Classless Static Routes option and
	//   a Router option, the DHCP client MUST ignore the Router option.
	if len(ack.ClasslessStaticRoute()) > 0 {
		// Track gateways for which we've already emitted an on-link route so
		// we don't add duplicates when several classless routes share a gateway.
		onLinkGateways := map[netip.Addr]struct{}{}

		for _, route := range ack.ClasslessStaticRoute() {
			gw, _ := netipx.FromStdIP(route.Router)
			dst, _ := netipx.FromStdIPNet(route.Dest)

			specs.Routes = append(specs.Routes, network.RouteSpecSpec{
				Family:      nethelpers.FamilyInet4,
				Destination: dst,
				Source:      addr.Addr(),
				Gateway:     gw,
				OutLinkName: linkName,
				Table:       nethelpers.TableMain,
				Priority:    routeMetric,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Protocol:    nethelpers.ProtocolBoot,
				ConfigLayer: network.ConfigOperator,
			})

			// If the gateway lives outside the lease's subnet, the kernel
			// can't resolve it as on-link and refuses to install the route.
			// AWS does this in IPv6-only subnets, handing out a 169.254.x.x/32
			// lease with classless routes via 169.254.0.1. Add an explicit
			// on-link route to the gateway so those routes can be installed.
			if gw.IsValid() && !gw.IsUnspecified() && !addr.Contains(gw) {
				if _, seen := onLinkGateways[gw]; !seen {
					onLinkGateways[gw] = struct{}{}

					specs.Routes = append(specs.Routes, network.RouteSpecSpec{
						Family:      nethelpers.FamilyInet4,
						Destination: netip.PrefixFrom(gw, gw.BitLen()),
						Source:      addr.Addr(),
						OutLinkName: linkName,
						Table:       nethelpers.TableMain,
						Priority:    routeMetric,
						Scope:       nethelpers.ScopeLink,
						Type:        nethelpers.TypeUnicast,
						Protocol:    nethelpers.ProtocolBoot,
						ConfigLayer: network.ConfigOperator,
					})
				}
			}
		}
	} else {
		for _, router := range ack.Router() {
			gw, _ := netipx.FromStdIP(router)

			specs.Routes = append(specs.Routes, network.RouteSpecSpec{
				Family:      nethelpers.FamilyInet4,
				Gateway:     gw,
				Source:      addr.Addr(),
				OutLinkName: linkName,
				Table:       nethelpers.TableMain,
				Priority:    routeMetric,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Protocol:    nethelpers.ProtocolBoot,
				ConfigLayer: network.ConfigOperator,
			})

			if !addr.Contains(gw) {
				// Add an interface route for the gateway if it's not in the same network
				specs.Routes = append(specs.Routes, network.RouteSpecSpec{
					Family:      nethelpers.FamilyInet4,
					Destination: netip.PrefixFrom(gw, gw.BitLen()),
					Source:      addr.Addr(),
					OutLinkName: linkName,
					Table:       nethelpers.TableMain,
					Priority:    routeMetric,
					Scope:       nethelpers.ScopeLink,
					Type:        nethelpers.TypeUnicast,
					Protocol:    nethelpers.ProtocolBoot,
					ConfigLayer: network.ConfigOperator,
				})
			}
		}
	}

	for i := range specs.Routes {
		specs.Routes[i].Normalize()
	}

	if useHostname {
		hostname := strings.TrimRight(ack.HostName(), "\x00")

		if hostname != "" {
			spec := network.HostnameSpecSpec{
				ConfigLayer: network.ConfigOperator,
			}

			if err := spec.ParseFQDN(hostname); err == nil {
				domainName := strings.TrimRight(ack.DomainName(), "\x00")

				if domainName != "" {
					spec.Domainname = domainName
				}

				specs.Hostname = []network.HostnameSpecSpec{
					spec,
				}
			}
		}
	}

	searchDomains := dhcpSearchDomains(ack)

	if len(ack.DNS()) > 0 || len(searchDomains) > 0 {
		convertIP := func(ip net.IP) netip.Addr {
			result, _ := netipx.FromStdIP(ip)

			return result
		}

		specs.Resolvers = []network.ResolverSpecSpec{
			{
				NameServers: xslices.Map(ack.DNS(), func(ip net.IP) network.NameServerSpec {
					return network.NameServerSpec{Addr: convertIP(ip)}
				}),
				SearchDomains: searchDomains,
				ConfigLayer:   network.ConfigOperator,
			},
		}
	}

	if len(ack.NTPServers()) > 0 {
		convertIP := func(ip net.IP) string {
			result, _ := netipx.FromStdIP(ip)

			return result.String()
		}

		specs.TimeServers = []network.TimeServerSpecSpec{
			{
				NTPServers:  xslices.Map(ack.NTPServers(), convertIP),
				ConfigLayer: network.ConfigOperator,
			},
		}
	}

	return specs
}

func dhcpSearchDomains(ack *dhcpv4.DHCPv4) []string {
	var searchDomains []string

	if labels := ack.DomainSearch(); labels != nil {
		searchDomains = append(searchDomains, labels.Labels...)
	}

	if domainName := strings.TrimRight(ack.DomainName(), "\x00"); domainName != "" && !slices.Contains(searchDomains, domainName) {
		searchDomains = append(searchDomains, domainName)
	}

	return searchDomains
}
