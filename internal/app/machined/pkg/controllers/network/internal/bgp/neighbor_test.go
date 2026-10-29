// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net/netip"
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sys/unix"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

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

func TestNeighborResolverAuthPasswordConflict(t *testing.T) {
	t.Parallel()

	runtimeState := internalbgp.NewRuntimeState(
		map[string]network.LinkStatusSpec{
			"eth0": {Index: 10},
			"eth1": {Index: 11},
			"eth2": {Index: 12},
		},
		nil,
	)

	// All three links see the same link-local router address, as is common with a shared ToR configuration.
	neighbors := []rtnetlink.NeighMessage{
		neighborMessage(10, 0, unix.NTF_ROUTER, "fe80::1"),
		neighborMessage(11, 0, unix.NTF_ROUTER, "fe80::1"),
		neighborMessage(12, 0, unix.NTF_ROUTER, "fe80::1"),
	}

	for _, test := range []struct {
		name      string
		passwords []string
		expected  []string
	}{
		{
			name:      "same password",
			passwords: []string{"secret", "secret", "secret"},
		},
		{
			name:      "different passwords",
			passwords: []string{"secret-a", "secret-b", "secret-a"},
			expected:  []string{"eth1", "eth2"},
		},
		{
			name:      "password on some links only",
			passwords: []string{"", "secret", ""},
			expected:  []string{"eth1", "eth2"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zapcore.WarnLevel)

			peers, peerIfaces := internalbgp.NewNeighborResolverForTest(neighbors).ResolvePeers(network.BGPInstanceConfigSpec{
				Neighbors: []network.BGPNeighborConfigSpec{
					{Link: "eth0", PeerASN: 65002, AuthPassword: test.passwords[0]},
					{Link: "eth1", PeerASN: 65002, AuthPassword: test.passwords[1]},
					{Link: "eth2", PeerASN: 65002, AuthPassword: test.passwords[2]},
				},
			}, runtimeState, zap.New(core))

			require.Len(t, peers, 3)
			assert.Equal(t, map[netip.Addr]string{netip.MustParseAddr("fe80::1"): "eth2"}, peerIfaces)

			links := xslices.Map(logs.All(), func(entry observer.LoggedEntry) string {
				assert.Contains(t, entry.Message, "different authPassword values")

				return entry.ContextMap()["link"].(string) //nolint:forcetypeassert
			})

			assert.Equal(t, test.expected, links)
		})
	}
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
