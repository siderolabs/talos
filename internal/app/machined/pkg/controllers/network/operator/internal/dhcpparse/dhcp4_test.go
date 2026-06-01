// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dhcpparse_test

import (
	"net"
	"net/netip"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator/internal/dhcpparse"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestParseDHCP4Ack(t *testing.T) {
	const (
		linkName    = "eth0"
		routeMetric = uint32(1024)
	)

	t.Run("basic ACK with router in lease subnet", func(t *testing.T) {
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptRouter(net.IPv4(10, 0, 0, 1))),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Addresses, 1)
		assert.Equal(t, must.Value(netip.ParsePrefix("10.0.0.5/24"))(t), specs.Addresses[0].Address)
		assert.Equal(t, linkName, specs.Addresses[0].LinkName)
		assert.Equal(t, nethelpers.FamilyInet4, specs.Addresses[0].Family)
		assert.Equal(t, routeMetric, specs.Addresses[0].Priority)

		assert.Empty(t, specs.Links, "no MTU option => no link spec")

		require.Len(t, specs.Routes, 1)
		assert.Equal(t, must.Value(netip.ParseAddr("10.0.0.1"))(t), specs.Routes[0].Gateway)
		assert.Equal(t, must.Value(netip.ParseAddr("10.0.0.5"))(t), specs.Routes[0].Source)
		assert.Equal(t, nethelpers.ScopeGlobal, specs.Routes[0].Scope)
		assert.Equal(t, linkName, specs.Routes[0].OutLinkName)

		assert.Empty(t, specs.Hostname)
		assert.Empty(t, specs.Resolvers)
		assert.Empty(t, specs.TimeServers)
	})

	t.Run("router outside lease subnet adds on-link route", func(t *testing.T) {
		// Lease 10.0.0.5/32 with gateway 10.0.0.1: gateway not on-link.
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(32, 32)),
			dhcpv4.WithOption(dhcpv4.OptRouter(net.IPv4(10, 0, 0, 1))),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Routes, 2)
		assert.Equal(t, must.Value(netip.ParseAddr("10.0.0.1"))(t), specs.Routes[0].Gateway)
		assert.Equal(t, nethelpers.ScopeGlobal, specs.Routes[0].Scope)

		assert.Equal(t, must.Value(netip.ParsePrefix("10.0.0.1/32"))(t), specs.Routes[1].Destination)
		assert.Equal(t, nethelpers.ScopeLink, specs.Routes[1].Scope, "on-link helper route")
		assert.False(t, specs.Routes[1].Gateway.IsValid(), "on-link route has no gateway")
	})

	t.Run("classless static routes win over router option", func(t *testing.T) {
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptRouter(net.IPv4(10, 0, 0, 1))),
			dhcpv4.WithOption(dhcpv4.OptClasslessStaticRoute(&dhcpv4.Route{
				Dest:   &net.IPNet{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(0, 32)},
				Router: net.IPv4(10, 0, 0, 254),
			})),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		// Classless routes win, router option ignored — only the classless
		// route is present (gateway is in the subnet, no helper needed).
		require.Len(t, specs.Routes, 1)
		assert.Equal(t, must.Value(netip.ParseAddr("10.0.0.254"))(t), specs.Routes[0].Gateway)
	})

	t.Run("classless route with gateway outside lease subnet adds on-link route (AWS IPv6-only)", func(t *testing.T) {
		// AWS IPv6-only handout: 169.254.x.x/32 lease, classless routes via
		// 169.254.0.1 — without an on-link helper the kernel refuses these
		// routes with ENETUNREACH.
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(169, 254, 251, 148)),
			dhcpv4.WithNetmask(net.CIDRMask(32, 32)),
			dhcpv4.WithOption(dhcpv4.OptClasslessStaticRoute(
				&dhcpv4.Route{
					Dest:   &net.IPNet{IP: net.IPv4(169, 254, 169, 123), Mask: net.CIDRMask(32, 32)},
					Router: net.IPv4(169, 254, 0, 1),
				},
				&dhcpv4.Route{
					Dest:   &net.IPNet{IP: net.IPv4(169, 254, 169, 249), Mask: net.CIDRMask(32, 32)},
					Router: net.IPv4(169, 254, 0, 1),
				},
				&dhcpv4.Route{
					Dest:   &net.IPNet{IP: net.IPv4(169, 254, 169, 254), Mask: net.CIDRMask(31, 32)},
					Router: net.IPv4(169, 254, 0, 1),
				},
			)),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		// 3 classless routes + 1 on-link helper for the shared gateway (deduped).
		require.Len(t, specs.Routes, 4)

		gw := must.Value(netip.ParseAddr("169.254.0.1"))(t)

		assert.Equal(t, must.Value(netip.ParsePrefix("169.254.169.123/32"))(t), specs.Routes[0].Destination)
		assert.Equal(t, gw, specs.Routes[0].Gateway)
		assert.Equal(t, must.Value(netip.ParsePrefix("169.254.0.1/32"))(t), specs.Routes[1].Destination)
		assert.Equal(t, nethelpers.ScopeLink, specs.Routes[1].Scope)
		assert.False(t, specs.Routes[1].Gateway.IsValid(), "on-link helper has no gateway")

		assert.Equal(t, must.Value(netip.ParsePrefix("169.254.169.249/32"))(t), specs.Routes[2].Destination)
		assert.Equal(t, gw, specs.Routes[2].Gateway)

		assert.Equal(t, must.Value(netip.ParsePrefix("169.254.169.254/31"))(t), specs.Routes[3].Destination)
		assert.Equal(t, gw, specs.Routes[3].Gateway)
	})

	t.Run("classless route with on-link router (router=0.0.0.0)", func(t *testing.T) {
		// RFC 3442: router=0.0.0.0 means destination is on-link.
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptClasslessStaticRoute(&dhcpv4.Route{
				Dest:   &net.IPNet{IP: net.IPv4(192, 168, 1, 0), Mask: net.CIDRMask(24, 32)},
				Router: net.IPv4(0, 0, 0, 0),
			})),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Routes, 1, "no helper for unspecified gateway")
		assert.Equal(t, must.Value(netip.ParsePrefix("192.168.1.0/24"))(t), specs.Routes[0].Destination)
		assert.False(t, specs.Routes[0].Gateway.IsValid(), "Normalize converts 0.0.0.0 to zero value")
		assert.Equal(t, nethelpers.ScopeLink, specs.Routes[0].Scope, "Normalize sets scope=link for routes with no gateway")
	})

	t.Run("DNS, NTP, MTU, hostname all parsed", func(t *testing.T) {
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptDNS(net.IPv4(8, 8, 8, 8), net.IPv4(8, 8, 4, 4))),
			dhcpv4.WithOption(dhcpv4.OptNTPServers(net.IPv4(169, 254, 169, 123))),
			dhcpv4.WithOption(dhcpv4.OptHostName("myhost")),
			dhcpv4.WithOption(dhcpv4.OptDomainName("example.com")),
			dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.OptionInterfaceMTU, []byte{0x05, 0xdc})), // 1500
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, true)

		require.Len(t, specs.Links, 1)
		assert.Equal(t, uint32(1500), specs.Links[0].MTU)
		assert.Equal(t, linkName, specs.Links[0].Name)
		assert.True(t, specs.Links[0].Up)

		require.Len(t, specs.Resolvers, 1)
		assert.Equal(
			t,
			[]network.NameServerSpec{
				{Addr: must.Value(netip.ParseAddr("8.8.8.8"))(t)},
				{Addr: must.Value(netip.ParseAddr("8.8.4.4"))(t)},
			},
			specs.Resolvers[0].NameServers,
		)
		assert.Equal(t, []string{"example.com"}, specs.Resolvers[0].SearchDomains,
			"DomainName feeds the search list when DomainSearch is absent")

		require.Len(t, specs.TimeServers, 1)
		assert.Equal(t, []string{"169.254.169.123"}, specs.TimeServers[0].NTPServers)

		require.Len(t, specs.Hostname, 1)
		assert.Equal(t, "myhost", specs.Hostname[0].Hostname)
		assert.Equal(t, "example.com", specs.Hostname[0].Domainname)
	})

	t.Run("search domains from DomainSearch option", func(t *testing.T) {
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptDNS(net.IPv4(8, 8, 8, 8))),
			dhcpv4.WithDomainSearchList("corp.example.com", "example.com"),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Resolvers, 1)
		assert.Equal(t, []string{"corp.example.com", "example.com"}, specs.Resolvers[0].SearchDomains)
	})

	t.Run("DomainName appended to DomainSearch when not already present", func(t *testing.T) {
		// Both options present, DomainName not in DomainSearch → appended.
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithDomainSearchList("corp.example.com"),
			dhcpv4.WithOption(dhcpv4.OptDomainName("example.com")),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Resolvers, 1)
		assert.Equal(t, []string{"corp.example.com", "example.com"}, specs.Resolvers[0].SearchDomains)
	})

	t.Run("DomainName not duplicated when already in DomainSearch", func(t *testing.T) {
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithDomainSearchList("example.com", "corp.example.com"),
			dhcpv4.WithOption(dhcpv4.OptDomainName("example.com")),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Resolvers, 1)
		assert.Equal(t, []string{"example.com", "corp.example.com"}, specs.Resolvers[0].SearchDomains)
	})

	t.Run("search domains alone still emit a resolver spec", func(t *testing.T) {
		// No DNS servers, just a DomainName — the resolver spec must still
		// be emitted so the search list reaches resolv.conf.
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptDomainName("example.com")),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Resolvers, 1)
		assert.Empty(t, specs.Resolvers[0].NameServers)
		assert.Equal(t, []string{"example.com"}, specs.Resolvers[0].SearchDomains)
	})

	t.Run("blank/null-padded DomainName does not become a search domain", func(t *testing.T) {
		// Some DHCP servers null-pad the DomainName option; after trimming
		// it should be treated as absent.
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptDNS(net.IPv4(8, 8, 8, 8))),
			dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.OptionDomainName, []byte("\x00\x00"))),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		require.Len(t, specs.Resolvers, 1)
		assert.Empty(t, specs.Resolvers[0].SearchDomains)
	})

	t.Run("useHostname=false ignores hostname even when present", func(t *testing.T) {
		ack := must.Value(dhcpv4.New(
			dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
			dhcpv4.WithYourIP(net.IPv4(10, 0, 0, 5)),
			dhcpv4.WithNetmask(net.CIDRMask(24, 32)),
			dhcpv4.WithOption(dhcpv4.OptHostName("myhost")),
		))(t)

		specs := dhcpparse.ParseDHCP4Ack(ack, linkName, routeMetric, false)

		assert.Empty(t, specs.Hostname)
	})
}
