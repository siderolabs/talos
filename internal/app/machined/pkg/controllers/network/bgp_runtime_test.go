// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBGPRuntimeStateResolvesAliasesAndAddresses(t *testing.T) {
	t.Parallel()

	vrf := network.NewLinkStatus(network.NamespaceName, "vrf-blue")
	vrf.TypedSpec().Index = 10
	vrf.TypedSpec().Kind = network.LinkKindVRF

	advertiseLink := network.NewLinkStatus(network.NamespaceName, "dummy0")
	advertiseLink.TypedSpec().Index = 11
	advertiseLink.TypedSpec().MasterIndex = 10
	advertiseLink.TypedSpec().Alias = "node-ip"

	peerLink := network.NewLinkStatus(network.NamespaceName, "eth1")
	peerLink.TypedSpec().Index = 12
	peerLink.TypedSpec().MasterIndex = 10
	peerLink.TypedSpec().AltNames = []string{"fabric0"}

	addresses := []*network.AddressStatus{
		newBGPAddressStatus("dummy0/10.0.0.2/32", "10.0.0.2/32", "dummy0", 11),
		newBGPAddressStatus("dummy0/127.0.0.1/8", "127.0.0.1/8", "dummy0", 11),
		newBGPAddressStatus("dummy0/fe80::1/64", "fe80::1/64", "dummy0", 11),
	}

	links := []*network.LinkStatus{vrf, advertiseLink, peerLink}

	resolved, prefixes, err := netctrl.ResolveBGPRuntimeSpecForTest(links, addresses, &network.BGPInstanceConfigSpec{
		LocalASN:       65001,
		RouterID:       netip.MustParseAddr("10.0.0.1"),
		RouteSource:    netip.MustParseAddr("10.0.0.2"),
		AdvertiseLinks: []string{"node-ip"},
		VRF:            "vrf-blue",
		VRFTable:       nethelpers.RoutingTable(88),
		Neighbors: []network.BGPNeighborConfigSpec{
			{Link: "fabric0", PeerASN: 65002},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "vrf-blue", resolved.VRF)
	require.Equal(t, []string{"dummy0"}, resolved.AdvertiseLinks)
	require.Equal(t, "eth1", resolved.Neighbors[0].Link)

	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.2/32")}, prefixes)

	_, _, err = netctrl.ResolveBGPRuntimeSpecForTest(links, addresses[1:], &resolved)
	require.ErrorContains(t, err, "route source: address 10.0.0.2 is not ready")
}

func newBGPAddressStatus(id, prefix, link string, index uint32) *network.AddressStatus {
	status := network.NewAddressStatus(network.NamespaceName, id)
	status.TypedSpec().Address = netip.MustParsePrefix(prefix)
	status.TypedSpec().LinkName = link
	status.TypedSpec().LinkIndex = index

	return status
}
