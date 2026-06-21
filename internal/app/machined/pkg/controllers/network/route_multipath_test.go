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

	a := []rtnetlink.NextHop{
		{Hop: rtnetlink.RTNextHop{IfIndex: 2}, Gateway: netip.MustParseAddr("10.5.0.1").AsSlice()},
		{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
	}

	assert.True(t, netctrl.MultipathEqualForTest(a, a))
	assert.False(t, netctrl.MultipathEqualForTest(a, a[:1]))

	b := []rtnetlink.NextHop{
		{Hop: rtnetlink.RTNextHop{IfIndex: 9}, Gateway: netip.MustParseAddr("10.5.0.1").AsSlice()},
		{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
	}
	assert.False(t, netctrl.MultipathEqualForTest(a, b))

	c := []rtnetlink.NextHop{
		{Hop: rtnetlink.RTNextHop{IfIndex: 2}, Gateway: netip.MustParseAddr("10.5.0.9").AsSlice()},
		{Hop: rtnetlink.RTNextHop{IfIndex: 3}, Gateway: netip.MustParseAddr("10.5.0.2").AsSlice()},
	}
	assert.False(t, netctrl.MultipathEqualForTest(a, c))
}
