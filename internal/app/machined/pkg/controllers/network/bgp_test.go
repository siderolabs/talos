// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/siderolabs/gen/value"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestRouteSpecSinglePath(t *testing.T) {
	t.Parallel()

	spec := netctrl.RouteSpecForTest(
		netip.MustParsePrefix("10.0.0.0/24"),
		[]network.RouteNextHop{
			{Gateway: netip.MustParseAddr("10.5.0.1")},
		},
		netip.Addr{},
	)

	assert.Equal(t, nethelpers.FamilyInet4, spec.Family)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), spec.Destination)
	assert.Equal(t, netip.MustParseAddr("10.5.0.1"), spec.Gateway)
	assert.Empty(t, spec.NextHops)
	assert.Equal(t, nethelpers.TableMain, spec.Table)
	assert.Equal(t, nethelpers.ProtocolBGP, spec.Protocol)
	assert.Equal(t, nethelpers.ScopeGlobal, spec.Scope)
}

func TestRouteSpecMultipath(t *testing.T) {
	t.Parallel()

	nexthops := []network.RouteNextHop{
		{Gateway: netip.MustParseAddr("10.5.0.1")},
		{Gateway: netip.MustParseAddr("10.5.0.2")},
	}
	spec := netctrl.RouteSpecForTest(
		netip.MustParsePrefix("2001:db8::/64"),
		nexthops,
		netip.Addr{},
	)

	assert.Equal(t, nethelpers.FamilyInet6, spec.Family)
	assert.True(t, value.IsZero(spec.Gateway), "top-level gateway must be unset for multipath")
	require.Len(t, spec.NextHops, 2)
	assert.Equal(t, netip.MustParseAddr("10.5.0.1"), spec.NextHops[0].Gateway)
	assert.Equal(t, netip.MustParseAddr("10.5.0.2"), spec.NextHops[1].Gateway)
}

func TestAddrFamily(t *testing.T) {
	t.Parallel()

	assert.Equal(t, nethelpers.FamilyInet4, netctrl.AddrFamilyForTest(netip.MustParseAddr("10.0.0.1")))
	assert.Equal(t, nethelpers.FamilyInet6, netctrl.AddrFamilyForTest(netip.MustParseAddr("2001:db8::1")))
	assert.Equal(t, nethelpers.FamilyInet6, netctrl.AddrFamilyForTest(netip.MustParseAddr("fe80::1")))
}

func TestToBGPSessionState(t *testing.T) {
	t.Parallel()

	for in, expected := range map[gobgpapi.PeerState_SessionState]nethelpers.BGPSessionState{
		gobgpapi.PeerState_SESSION_STATE_IDLE:        nethelpers.BGPSessionStateIdle,
		gobgpapi.PeerState_SESSION_STATE_CONNECT:     nethelpers.BGPSessionStateConnect,
		gobgpapi.PeerState_SESSION_STATE_ACTIVE:      nethelpers.BGPSessionStateActive,
		gobgpapi.PeerState_SESSION_STATE_OPENSENT:    nethelpers.BGPSessionStateOpenSent,
		gobgpapi.PeerState_SESSION_STATE_OPENCONFIRM: nethelpers.BGPSessionStateOpenConfirm,
		gobgpapi.PeerState_SESSION_STATE_ESTABLISHED: nethelpers.BGPSessionStateEstablished,
		gobgpapi.PeerState_SESSION_STATE_UNSPECIFIED: nethelpers.BGPSessionStateUnknown,
	} {
		assert.Equal(t, expected, netctrl.BGPSessionStateForTest(in))
	}
}

func TestPathNexthopIPv4(t *testing.T) {
	t.Parallel()

	nexthop, err := bgppacket.NewPathAttributeNextHop(netip.MustParseAddr("10.5.0.1"))
	require.NoError(t, err)

	path := &apiutil.Path{Attrs: []bgppacket.PathAttributeInterface{bgppacket.NewPathAttributeOrigin(0), nexthop}}

	assert.Equal(t, netip.MustParseAddr("10.5.0.1"), netctrl.PathNexthopForTest(path))
}

func TestPathNexthopMpReach(t *testing.T) {
	t.Parallel()

	nlri, err := bgppacket.NewIPAddrPrefix(netip.MustParsePrefix("2001:db8::/64"))
	require.NoError(t, err)

	mpReach, err := bgppacket.NewPathAttributeMpReachNLRI(bgppacket.RF_IPv6_UC, []bgppacket.PathNLRI{{NLRI: nlri}}, netip.MustParseAddr("2001:db8::1"))
	require.NoError(t, err)

	path := &apiutil.Path{Attrs: []bgppacket.PathAttributeInterface{bgppacket.NewPathAttributeOrigin(0), mpReach}}

	assert.Equal(t, netip.MustParseAddr("2001:db8::1"), netctrl.PathNexthopForTest(path))
}

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
