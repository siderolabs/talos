// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBuildMultipath(t *testing.T) {
	t.Parallel()

	links := []rtnetlink.LinkMessage{
		{Index: 2, Attributes: &rtnetlink.LinkAttributes{Name: "eth0"}},
		{Index: 3, Attributes: &rtnetlink.LinkAttributes{Name: "eth1"}},
	}

	t.Run("resolves links and weights", func(t *testing.T) {
		t.Parallel()

		multipath, ok := netctrl.BuildMultipathForTest(nethelpers.FamilyInet4, links, []network.RouteNextHop{
			{Gateway: netip.MustParseAddr("10.5.0.1"), OutLinkName: "eth0"},
			{Gateway: netip.MustParseAddr("10.5.0.2"), OutLinkName: "eth1", Weight: 3},
		})

		require.True(t, ok)
		require.Len(t, multipath, 2)

		assert.Equal(t, uint32(2), multipath[0].Hop.IfIndex)
		assert.Equal(t, uint8(0), multipath[0].Hop.Hops) // default weight 1 -> hops 0
		assert.Equal(t, uint32(3), multipath[1].Hop.IfIndex)
		assert.Equal(t, uint8(2), multipath[1].Hop.Hops) // weight 3 -> hops 2
	})

	t.Run("missing link returns false", func(t *testing.T) {
		t.Parallel()

		_, ok := netctrl.BuildMultipathForTest(nethelpers.FamilyInet4, links, []network.RouteNextHop{
			{Gateway: netip.MustParseAddr("10.5.0.1"), OutLinkName: "missing0"},
		})

		assert.False(t, ok)
	})

	t.Run("no out-link resolves to index 0", func(t *testing.T) {
		t.Parallel()

		multipath, ok := netctrl.BuildMultipathForTest(nethelpers.FamilyInet4, links, []network.RouteNextHop{
			{Gateway: netip.MustParseAddr("10.5.0.1")},
		})

		require.True(t, ok)
		require.Len(t, multipath, 1)
		assert.Equal(t, uint32(0), multipath[0].Hop.IfIndex)
	})

	t.Run("installs next-hops in canonical order without duplicates", func(t *testing.T) {
		t.Parallel()

		multipath, ok := netctrl.BuildMultipathForTest(nethelpers.FamilyInet4, links, []network.RouteNextHop{
			{Gateway: netip.MustParseAddr("10.5.0.2"), OutLinkName: "eth1"},
			{Gateway: netip.MustParseAddr("10.5.0.1"), OutLinkName: "eth0"},
			{Gateway: netip.MustParseAddr("10.5.0.2"), OutLinkName: "eth1"},
		})

		require.True(t, ok)
		require.Len(t, multipath, 2)

		assert.True(t, multipath[0].Gateway.Equal(netip.MustParseAddr("10.5.0.1").AsSlice()))
		assert.True(t, multipath[1].Gateway.Equal(netip.MustParseAddr("10.5.0.2").AsSlice()))
	})

	t.Run("cross-family IPv4-via-IPv6-LLA uses RTA_VIA (RFC 8950)", func(t *testing.T) {
		t.Parallel()

		lla := netip.MustParseAddr("fe80::1")

		multipath, ok := netctrl.BuildMultipathForTest(nethelpers.FamilyInet4, links, []network.RouteNextHop{
			{Gateway: lla, OutLinkName: "eth0"},
		})

		require.True(t, ok)
		require.Len(t, multipath, 1)
		assert.Nil(t, multipath[0].Gateway, "cross-family next-hop must not set RTA_GATEWAY")
		require.NotNil(t, multipath[0].Via, "cross-family next-hop must set RTA_VIA")
		assert.Equal(t, uint16(unix.AF_INET6), multipath[0].Via.Family)
		assert.True(t, multipath[0].Via.Addr.Equal(lla.AsSlice()))
		assert.Equal(t, uint32(2), multipath[0].Hop.IfIndex)
	})
}

func TestMultipathEqual(t *testing.T) {
	t.Parallel()

	expected := []rtnetlink.NextHop{
		{Hop: rtnetlink.RTNextHop{IfIndex: 2}, Gateway: netip.MustParseAddr("10.5.0.1").AsSlice()},
		{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
	}

	for _, test := range []struct {
		name     string
		existing []rtnetlink.NextHop
		expected []rtnetlink.NextHop
		equal    bool
	}{
		{
			name:     "identical",
			existing: expected,
			expected: expected,
			equal:    true,
		},
		{
			name:     "different length",
			existing: expected,
			expected: expected[:1],
		},
		{
			// the kernel keeps the next-hops in the order they were installed, which is not necessarily
			// the order the spec lists them in
			name:     "reordered",
			existing: []rtnetlink.NextHop{expected[1], expected[0]},
			expected: expected,
			equal:    true,
		},
		{
			// an existing next-hop may satisfy only one expected next-hop
			name:     "duplicate existing next-hop",
			existing: []rtnetlink.NextHop{expected[0], expected[0]},
			expected: expected,
		},
		{
			name:     "different link",
			existing: expected,
			expected: []rtnetlink.NextHop{
				{Hop: rtnetlink.RTNextHop{IfIndex: 9}, Gateway: netip.MustParseAddr("10.5.0.1").AsSlice()},
				{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
			},
		},
		{
			name:     "different gateway",
			existing: expected,
			expected: []rtnetlink.NextHop{
				{Hop: rtnetlink.RTNextHop{IfIndex: 2}, Gateway: netip.MustParseAddr("10.5.0.9").AsSlice()},
				{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
			},
		},
		{
			name:     "different weight",
			existing: expected,
			expected: []rtnetlink.NextHop{
				{Hop: rtnetlink.RTNextHop{IfIndex: 2, Hops: 1}, Gateway: netip.MustParseAddr("10.5.0.1").AsSlice()},
				{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
			},
		},
		{
			// a spec without out-links (numbered BGP peers) must match the interfaces the kernel
			// resolved on its own, otherwise the route is rewritten on every reconcile
			name:     "no out-link matches any resolved link",
			existing: expected,
			expected: []rtnetlink.NextHop{
				{Gateway: netip.MustParseAddr("10.5.0.1").AsSlice()},
				{Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
			},
			equal: true,
		},
		{
			// the wildcard applies to the out-link only, the gateway still has to match
			name:     "no out-link still compares gateways",
			existing: expected,
			expected: []rtnetlink.NextHop{
				{Gateway: netip.MustParseAddr("10.5.0.1").AsSlice()},
				{Gateway: netip.MustParseAddr("10.5.0.9").AsSlice()},
			},
		},
		{
			name: "no out-link still compares via",
			existing: []rtnetlink.NextHop{
				{Hop: rtnetlink.RTNextHop{IfIndex: 2}, Via: &rtnetlink.RouteVia{
					Family: unix.AF_INET6,
					Addr:   netip.MustParseAddr("fe80::1").AsSlice(),
				}},
			},
			expected: []rtnetlink.NextHop{
				{Via: &rtnetlink.RouteVia{
					Family: unix.AF_INET6,
					Addr:   netip.MustParseAddr("fe80::2").AsSlice(),
				}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.equal, netctrl.MultipathEqualForTest(test.existing, test.expected))
		})
	}
}

// TestNumberedNextHopsNoChurn covers the full path a BGP route learned from numbered (non-link-local)
// peers takes: such next-hops carry no out-link, the kernel resolves and reports one, and the result
// must still compare equal to the spec.
//
// See https://github.com/siderolabs/talos/issues/14416.
func TestNumberedNextHopsNoChurn(t *testing.T) {
	t.Parallel()

	links := []rtnetlink.LinkMessage{
		{Index: 2, Attributes: &rtnetlink.LinkAttributes{Name: "eth0"}},
		{Index: 3, Attributes: &rtnetlink.LinkAttributes{Name: "eth1"}},
	}

	// a numbered peer leaves OutLinkName empty, only link-local (unnumbered) peers set it
	spec := bgp.RouteSpec(
		netip.MustParsePrefix("0.0.0.0/0"),
		[]network.RouteNextHop{
			{Gateway: netip.MustParseAddr("192.0.2.0")},
			{Gateway: netip.MustParseAddr("192.0.2.2")},
		},
		netip.Addr{},
		nethelpers.TableMain,
	)
	require.Len(t, spec.NextHops, 2)

	multipath, ok := netctrl.BuildMultipathForTest(spec.Family, links, spec.NextHops)
	require.True(t, ok)

	// what the kernel reports back after resolving the next-hops itself
	kernel := []rtnetlink.NextHop{
		{Hop: rtnetlink.RTNextHop{IfIndex: 2}, Gateway: netip.MustParseAddr("192.0.2.0").AsSlice()},
		{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("192.0.2.2").AsSlice()},
	}

	assert.True(t, netctrl.MultipathEqualForTest(kernel, multipath),
		"a route from numbered peers must match what the kernel reports, or it is re-created on every reconcile")
}
