// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net/netip"
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestNumberedBFDSource(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, remote, local string
	}{
		{"IPv4", "192.0.2.2", "192.0.2.1/24"},
		{"IPv6", "2001:db8::2", "2001:db8::1/64"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := internalbgp.NewRuntimeState(map[string]network.LinkStatusSpec{
				"eth0": {Index: 2},
				"lo":   {Index: 1},
			}, []network.AddressStatusSpec{
				{LinkName: "lo", LinkIndex: 1, Address: netip.MustParsePrefix("2001:db8::ffff/128")},
				{LinkName: "eth0", LinkIndex: 2, Address: netip.MustParsePrefix(tt.local)},
			})
			peers, _ := internalbgp.NewNeighborResolver().ResolvePeers(network.BGPInstanceConfigSpec{
				Neighbors: []network.BGPNeighborConfigSpec{{Address: netip.MustParseAddr(tt.remote), PeerASN: 65002}},
			}, state, zap.NewNop())
			require.Len(t, peers, 1)
			assert.Equal(t, netip.MustParsePrefix(tt.local).Addr().String(), internalbgp.BuildPeer(peers[0], false).GetTransport().GetLocalAddress())
		})
	}
}

func TestNumberedSourceRoutingDomain(t *testing.T) {
	t.Parallel()

	addresses := []network.AddressStatusSpec{
		{LinkName: "eth0", LinkIndex: 2, Address: netip.MustParsePrefix("192.0.2.20/24")},
		{LinkName: "eth0", LinkIndex: 2, Address: netip.MustParsePrefix("192.0.2.10/24")},
		{LinkName: "eth0", LinkIndex: 2, Address: netip.MustParsePrefix("192.0.2.40/25")},
		{LinkName: "eth0", LinkIndex: 2, Address: netip.MustParsePrefix("192.0.2.1/30"), Flags: nethelpers.AddressFlags(nethelpers.AddressTentative)},
		{LinkName: "eth0", LinkIndex: 2, Address: netip.MustParsePrefix("192.0.2.3/30"), Flags: nethelpers.AddressFlags(nethelpers.AddressDADFailed)},
		{LinkName: "eth0", LinkIndex: 2, Address: netip.MustParsePrefix("192.0.2.4/29"), Flags: nethelpers.AddressFlags(nethelpers.AddressDeprecated)},
		{LinkName: "eth1", LinkIndex: 3, Address: netip.MustParsePrefix("192.0.2.30/25")},
	}
	links := map[string]network.LinkStatusSpec{
		"eth0":     {Index: 2},
		"eth1":     {Index: 3, MasterIndex: 4},
		"vrf-blue": {Index: 4, Kind: network.LinkKindVRF},
	}

	for _, tt := range []struct{ name, vrf, remote, want string }{
		{"most specific ready prefix", "", "192.0.2.2", "192.0.2.40"},
		{"default domain tie break", "", "192.0.2.200", "192.0.2.10"},
		{"VRF domain", "vrf-blue", "192.0.2.2", "192.0.2.30"},
		{"routed peer", "", "198.51.100.1", ""},
		{"wrong family", "", "2001:db8::2", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			peers, _ := internalbgp.NewNeighborResolver().ResolvePeers(network.BGPInstanceConfigSpec{
				VRF:       tt.vrf,
				Neighbors: []network.BGPNeighborConfigSpec{{Address: netip.MustParseAddr(tt.remote), PeerASN: 65002}},
			}, internalbgp.NewRuntimeState(links, addresses), zap.NewNop())
			require.Len(t, peers, 1)
			assert.Equal(t, tt.want, internalbgp.BuildPeer(peers[0], false).GetTransport().GetLocalAddress())
			oldKey := internalbgp.PeerKey(peers[0])
			peers[0].LocalAddress = "192.0.2.99"
			assert.NotEqual(t, oldKey, internalbgp.PeerKey(peers[0]))
		})
	}
}

func TestLinkLocalNeighbor(t *testing.T) {
	t.Parallel()

	valid := neighborMessage(10, 0, unix.NTF_ROUTER, "fe80::2")

	neighbors := []rtnetlink.NeighMessage{
		neighborMessage(11, 0, unix.NTF_ROUTER, "fe80::3"),
		{Index: 10},
		neighborMessage(10, unix.NUD_FAILED, unix.NTF_ROUTER, "fe80::4"),
		neighborMessage(10, unix.NUD_INCOMPLETE, unix.NTF_ROUTER, "fe80::5"),
		neighborMessage(10, 0, 0, "fe80::6"),
		neighborMessage(10, 0, unix.NTF_ROUTER, "192.0.2.1"),
		neighborMessage(10, 0, unix.NTF_ROUTER, "fe80::1"),
		valid,
	}

	addr, ok := internalbgp.SelectLinkLocalNeighbor(
		neighbors,
		"eth0",
		10,
		map[netip.Addr]struct{}{netip.MustParseAddr("fe80::1"): {}},
		zap.NewNop(),
	)
	require.True(t, ok)
	assert.Equal(t, netip.MustParseAddr("fe80::2"), addr)

	_, ok = internalbgp.SelectLinkLocalNeighbor(append(neighbors, neighborMessage(10, 0, unix.NTF_ROUTER, "fe80::7")), "eth0", 10, nil, zap.NewNop())
	assert.False(t, ok, "multiple router neighbors must remain unresolved")

	_, ok = internalbgp.SelectLinkLocalNeighbor(nil, "eth0", 10, nil, zap.NewNop())
	assert.False(t, ok, "an empty snapshot must remain unresolved")
}

func TestNeighborResolver(t *testing.T) {
	t.Parallel()

	runtimeState := internalbgp.NewRuntimeState(
		map[string]network.LinkStatusSpec{
			"vrf-blue": {Index: 20, Kind: network.LinkKindVRF},
			"eth0":     {Index: 10, MasterIndex: 20},
		},
		[]network.AddressStatusSpec{
			{Address: netip.MustParsePrefix("fe80::1/64"), LinkName: "eth0", LinkIndex: 10},
		},
	)

	peer, ok := internalbgp.ResolvePeer(
		network.BGPNeighborConfigSpec{Link: "eth0", PeerASN: 65002},
		runtimeState,
		[]rtnetlink.NeighMessage{neighborMessage(10, 0, unix.NTF_ROUTER, "fe80::2")},
		zap.NewNop(),
	)
	require.True(t, ok)
	assert.Equal(t, "fe80::2%eth0", peer.Address)
	assert.Equal(t, netip.MustParseAddr("fe80::2"), peer.LinkLocal)
	assert.Equal(t, "eth0", peer.Link)

	peers, _ := internalbgp.NewNeighborResolver().ResolvePeers(network.BGPInstanceConfigSpec{
		VRF: "vrf-blue",
		Neighbors: []network.BGPNeighborConfigSpec{
			{Address: netip.MustParseAddr("192.0.2.2"), PeerASN: 65002},
		},
	}, runtimeState, zap.NewNop())
	require.Len(t, peers, 1)
	assert.Equal(t, "vrf-blue", peers[0].BindInterface)
}

func TestNeighborResolverNumberedPeerSkipsRtnetlink(t *testing.T) {
	t.Parallel()

	resolver := internalbgp.NewNeighborResolver()

	peers, _ := resolver.ResolvePeers(network.BGPInstanceConfigSpec{
		Neighbors: []network.BGPNeighborConfigSpec{
			{Address: netip.MustParseAddr("192.0.2.2"), PeerASN: 65002},
		},
	}, internalbgp.NewRuntimeState(nil, nil), zap.NewNop())

	require.Len(t, peers, 1)
	assert.Equal(t, "192.0.2.2", peers[0].Address)

	_, ok := internalbgp.ResolvePeer(network.BGPNeighborConfigSpec{}, internalbgp.NewRuntimeState(nil, nil), nil, zap.NewNop())
	assert.False(t, ok)
}

func neighborMessage(index uint32, state uint16, flags uint8, address string) rtnetlink.NeighMessage {
	return rtnetlink.NeighMessage{
		Index: index,
		State: state,
		Flags: flags,
		Attributes: &rtnetlink.NeighAttributes{
			Address: netip.MustParseAddr(address).AsSlice(),
		},
	}
}
