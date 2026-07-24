// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package mgmt_test

import (
	"net/netip"
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt"
)

func TestFabricOwnedRouteInventory(t *testing.T) {
	t.Parallel()

	destination := netip.MustParsePrefix("172.16.0.2/32")
	fabricIfaces := map[uint32]struct{}{10: {}, 11: {}}

	ownedSingle := fabricTestRoute(destination, unix.RTPROT_BGP, 10)
	ownedMultipath := fabricTestRoute(netip.MustParsePrefix("198.51.100.100/32"), unix.RTPROT_BGP, 0)
	ownedMultipath.Attributes.Multipath = []rtnetlink.NextHop{
		{Hop: rtnetlink.RTNextHop{IfIndex: 10}},
		{Hop: rtnetlink.RTNextHop{IfIndex: 11}},
	}

	inventory := mgmt.BGPLaunchOwnedRouteInventoryForTest([]rtnetlink.RouteMessage{
		ownedSingle,
		ownedMultipath,
		fabricTestRoute(destination, unix.RTPROT_STATIC, 10),
		fabricTestRoute(netip.MustParsePrefix("172.16.0.3/32"), unix.RTPROT_BGP, 12),
		fabricTestRoute(netip.MustParsePrefix("10.244.0.0/24"), unix.RTPROT_BGP, 10),
	}, fabricIfaces)

	require.Len(t, inventory, 2)
	assert.Equal(t, ownedSingle, inventory[destination])
	assert.Equal(t, ownedMultipath, inventory[netip.MustParsePrefix("198.51.100.100/32")])
}

func TestFabricOwnedRouteRejectsMixedMultipath(t *testing.T) {
	t.Parallel()

	route := fabricTestRoute(netip.MustParsePrefix("172.16.0.2/32"), unix.RTPROT_BGP, 0)
	route.Attributes.Multipath = []rtnetlink.NextHop{
		{Hop: rtnetlink.RTNextHop{IfIndex: 10}},
		{Hop: rtnetlink.RTNextHop{IfIndex: 12}},
	}

	_, ok := mgmt.BGPLaunchOwnedRoutePrefixForTest(route, map[uint32]struct{}{10: {}, 11: {}})
	assert.False(t, ok, "a BGP route using another process's interface must remain untouched")
}

func fabricTestRoute(destination netip.Prefix, protocol uint8, outIface uint32) rtnetlink.RouteMessage {
	return rtnetlink.RouteMessage{
		Family:    unix.AF_INET,
		DstLength: uint8(destination.Bits()),
		Protocol:  protocol,
		Scope:     unix.RT_SCOPE_UNIVERSE,
		Type:      unix.RTN_UNICAST,
		Attributes: rtnetlink.RouteAttributes{
			Dst:      destination.Addr().AsSlice(),
			Table:    unix.RT_TABLE_MAIN,
			OutIface: outIface,
		},
	}
}
