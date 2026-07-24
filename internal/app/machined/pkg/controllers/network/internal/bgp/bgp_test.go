// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net/netip"
	"testing"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/siderolabs/gen/value"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBuildPeer(t *testing.T) {
	t.Parallel()

	peer := internalbgp.BuildPeer(internalbgp.Peer{
		Address:       "fe80::1%eth0",
		BindInterface: "vrf-blue",
		Config: network.BGPNeighborConfigSpec{
			PeerASN:  65002,
			LocalASN: 65003,
			Passive:  true,
			HoldTime: 90 * time.Second,
			BFD: &network.BGPBFDConfigSpec{
				TransmitInterval: 300 * time.Millisecond,
				ReceiveInterval:  400 * time.Millisecond,
				DetectMultiplier: 3,
			},
		},
	}, true)

	assert.Equal(t, "fe80::1%eth0", peer.GetConf().GetNeighborAddress())
	assert.Equal(t, uint32(65002), peer.GetConf().GetPeerAsn())
	assert.Equal(t, uint32(65003), peer.GetConf().GetLocalAsn())
	assert.True(t, peer.GetConf().GetReplacePeerAsn())
	assert.True(t, peer.GetTransport().GetPassiveMode())
	assert.Equal(t, "vrf-blue", peer.GetTransport().GetBindInterface())
	assert.Equal(t, uint64(90), peer.GetTimers().GetConfig().GetHoldTime())
	assert.Equal(t, uint64(30), peer.GetTimers().GetConfig().GetKeepaliveInterval())
	assert.Equal(t, uint32(300000), peer.GetBfd().GetDesiredMinimumTxInterval())
	assert.Equal(t, uint32(400000), peer.GetBfd().GetRequiredMinimumReceive())
	assert.Equal(t, uint32(3), peer.GetBfd().GetDetectionMultiplier())
	require.Len(t, peer.GetAfiSafis(), 2)
	assert.True(t, peer.GetAfiSafis()[0].GetUseMultiplePaths().GetConfig().GetEnabled())
	assert.True(t, peer.GetAfiSafis()[1].GetUseMultiplePaths().GetConfig().GetEnabled())
}

func TestBuildOriginatedPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		prefix netip.Prefix
		family bgppacket.Family
	}{
		{prefix: netip.MustParsePrefix("10.0.0.1/32"), family: bgppacket.RF_IPv4_UC},
		{prefix: netip.MustParsePrefix("2001:db8::1/128"), family: bgppacket.RF_IPv6_UC},
	} {
		path, err := internalbgp.BuildOriginatedPath(test.prefix)
		require.NoError(t, err)

		assert.Equal(t, test.family, path.Family)
		assert.Equal(t, test.prefix.String(), path.Nlri.String())
	}
}

func TestPathNexthop(t *testing.T) {
	t.Parallel()

	v4, err := bgppacket.NewPathAttributeNextHop(netip.MustParseAddr("10.5.0.1"))
	require.NoError(t, err)

	assert.Equal(t, netip.MustParseAddr("10.5.0.1"), internalbgp.PathNexthop(&apiutil.Path{
		Attrs: []bgppacket.PathAttributeInterface{bgppacket.NewPathAttributeOrigin(0), v4},
	}))

	nlri, err := bgppacket.NewIPAddrPrefix(netip.MustParsePrefix("2001:db8::/64"))
	require.NoError(t, err)

	mpReach, err := bgppacket.NewPathAttributeMpReachNLRI(
		bgppacket.RF_IPv6_UC,
		[]bgppacket.PathNLRI{{NLRI: nlri}},
		netip.MustParseAddr("2001:db8::1"),
	)
	require.NoError(t, err)

	mpReach.LinkLocalNexthop = netip.MustParseAddr("fe80::1")

	assert.Equal(t, netip.MustParseAddr("fe80::1"), internalbgp.PathNexthop(&apiutil.Path{
		Attrs: []bgppacket.PathAttributeInterface{bgppacket.NewPathAttributeOrigin(0), mpReach},
	}))
}

func TestRouteSpec(t *testing.T) {
	t.Parallel()

	single := internalbgp.RouteSpec(
		netip.MustParsePrefix("10.0.0.0/24"),
		[]network.RouteNextHop{{Gateway: netip.MustParseAddr("10.5.0.1")}},
		netip.MustParseAddr("10.0.0.1"),
		88,
	)

	assert.Equal(t, nethelpers.FamilyInet4, single.Family)
	assert.Equal(t, netip.MustParseAddr("10.5.0.1"), single.Gateway)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), single.Source)
	assert.Empty(t, single.NextHops)
	assert.Equal(t, nethelpers.ProtocolBGP, single.Protocol)
	assert.Equal(t, nethelpers.RoutingTable(88), single.Table)

	nexthops := []network.RouteNextHop{
		{Gateway: netip.MustParseAddr("fe80::1")},
		{Gateway: netip.MustParseAddr("fe80::2")},
	}
	multipath := internalbgp.RouteSpec(netip.MustParsePrefix("2001:db8::/64"), nexthops, netip.MustParseAddr("10.0.0.1"), nethelpers.TableUnspec)

	assert.Equal(t, nethelpers.FamilyInet6, multipath.Family)
	assert.True(t, value.IsZero(multipath.Gateway))
	assert.False(t, multipath.Source.IsValid(), "a cross-family preferred source must be ignored")
	assert.Equal(t, nexthops, multipath.NextHops)
	assert.Equal(t, nethelpers.TableMain, multipath.Table)
}

func TestSessionState(t *testing.T) {
	t.Parallel()

	for input, expected := range map[gobgpapi.PeerState_SessionState]nethelpers.BGPSessionState{
		gobgpapi.PeerState_SESSION_STATE_IDLE:        nethelpers.BGPSessionStateIdle,
		gobgpapi.PeerState_SESSION_STATE_CONNECT:     nethelpers.BGPSessionStateConnect,
		gobgpapi.PeerState_SESSION_STATE_ACTIVE:      nethelpers.BGPSessionStateActive,
		gobgpapi.PeerState_SESSION_STATE_OPENSENT:    nethelpers.BGPSessionStateOpenSent,
		gobgpapi.PeerState_SESSION_STATE_OPENCONFIRM: nethelpers.BGPSessionStateOpenConfirm,
		gobgpapi.PeerState_SESSION_STATE_ESTABLISHED: nethelpers.BGPSessionStateEstablished,
		gobgpapi.PeerState_SESSION_STATE_UNSPECIFIED: nethelpers.BGPSessionStateUnknown,
	} {
		assert.Equal(t, expected, internalbgp.SessionState(input))
	}
}

func TestPeerStatus(t *testing.T) {
	t.Parallel()

	status := internalbgp.PeerStatus(&gobgpapi.Peer{
		Conf: &gobgpapi.PeerConf{NeighborAddress: "192.0.2.1"},
		State: &gobgpapi.PeerState{
			PeerAsn:      65002,
			RouterId:     "192.0.2.2",
			SessionState: gobgpapi.PeerState_SESSION_STATE_ESTABLISHED,
			BfdState: &gobgpapi.BfdPeerState{
				SessionState: gobgpapi.BfdSessionState_BFD_SESSION_STATE_UP,
			},
		},
		AfiSafis: []*gobgpapi.AfiSafi{
			{State: &gobgpapi.AfiSafiState{Received: 5, Accepted: 4, Advertised: 3}},
			{State: &gobgpapi.AfiSafiState{Received: 7, Accepted: 6, Advertised: 2}},
		},
	}, 65001)

	assert.Equal(t, "192.0.2.1", status.Peer)
	assert.Equal(t, uint32(65001), status.LocalASN)
	assert.Equal(t, uint32(65002), status.PeerASN)
	assert.Equal(t, nethelpers.BGPSessionStateEstablished, status.State)
	assert.Equal(t, netip.MustParseAddr("192.0.2.2"), status.RouterID)
	assert.Equal(t, uint32(12), status.Received)
	assert.Equal(t, uint32(10), status.Accepted)
	assert.Equal(t, uint32(5), status.Advertised)
	assert.Equal(t, "up", status.BFDState)
}

func TestKeys(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		"asn=65001;router=10.0.0.1;multipath=true;maxpaths=8;vrf=vrf-blue;table=88;listen=179;",
		internalbgp.ServerKey(65001, netip.MustParseAddr("10.0.0.1"), true, 8, "vrf-blue", 88, 179),
	)

	peer := internalbgp.Peer{
		Address: "10.0.0.2",
		Config:  network.BGPNeighborConfigSpec{PeerASN: 65002, HoldTime: 90 * time.Second},
	}
	assert.NotEqual(t, internalbgp.PeerKey(peer), internalbgp.PeerKey(internalbgp.Peer{
		Address: peer.Address,
		Config: network.BGPNeighborConfigSpec{
			PeerASN:  peer.Config.PeerASN,
			HoldTime: peer.Config.HoldTime,
			BFD:      &network.BGPBFDConfigSpec{DetectMultiplier: 3},
		},
	}))
}
