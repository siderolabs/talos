// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net/netip"
	"testing"

	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	resourcenetwork "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// TestLearnedRouteOutLinks pins the out-link contract of learned routes: only a link-local
// (unnumbered) next-hop resolves to the peer interface, a numbered next-hop leaves the egress device
// to the kernel, which resolves it from the gateway.
//
// RouteSpecController has to tolerate the resulting empty out-link, otherwise it re-creates such
// routes on every reconcile: see https://github.com/siderolabs/talos/issues/14416.
//
// A link-local next-hop whose peer interface is unknown is dropped: the kernel can't install a
// link-local gateway without a device, so such a route would fail on every reconcile instead.
func TestLearnedRouteOutLinks(t *testing.T) {
	ctx := t.Context()
	server := startBGPImportTestServer(t, ctx, 65000, "192.0.2.1", -1, false)

	numbered := netip.MustParsePrefix("198.51.100.0/24")
	numberedNexthop := netip.MustParseAddr("192.0.2.10")
	unnumbered := netip.MustParsePrefix("2001:db8:1::/64")
	unnumberedNexthop := netip.MustParseAddr("fe80::2")
	unresolvable := netip.MustParsePrefix("2001:db8:2::/64")
	unresolvableNexthop := netip.MustParseAddr("fe80::3")

	responses, err := server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{
		newBGPImportTestPath(t, numbered, 50),
		newBGPLinkLocalTestPath(t, unnumbered, unnumberedNexthop),
		newBGPLinkLocalTestPath(t, unresolvable, unresolvableNexthop),
	}})
	require.NoError(t, err)
	require.Len(t, responses, 3)

	for _, response := range responses {
		require.NoError(t, response.Error)
	}

	instance := internalbgp.NewInstance()
	internalbgp.SetInstanceServerForTest(instance, server)
	internalbgp.SetInstancePeerIfacesForTest(instance, map[netip.Addr]string{unnumberedNexthop: "eth0"})
	instance.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{InstallRoutes: true})

	learned := instance.Snapshot(ctx).Learned

	assert.Equal(
		t,
		[]resourcenetwork.RouteNextHop{{Gateway: numberedNexthop}},
		learned[numbered],
		"a numbered next-hop must not carry an out-link",
	)

	assert.Equal(
		t,
		[]resourcenetwork.RouteNextHop{{Gateway: unnumberedNexthop, OutLinkName: "eth0"}},
		learned[unnumbered],
		"a link-local next-hop must resolve to the peer interface",
	)

	assert.NotContains(t, learned, unresolvable, "a link-local next-hop without a known peer interface must be dropped")
}

// newBGPLinkLocalTestPath builds a path as advertised by an unnumbered peer: both the next-hop and
// the peer address are the peer's link-local address.
func newBGPLinkLocalTestPath(t *testing.T, prefix netip.Prefix, linkLocal netip.Addr) *apiutil.Path {
	t.Helper()

	nlri, err := bgppacket.NewIPAddrPrefix(prefix)
	require.NoError(t, err)

	mpReach, err := bgppacket.NewPathAttributeMpReachNLRI(
		bgppacket.RF_IPv6_UC,
		[]bgppacket.PathNLRI{{NLRI: nlri}},
		linkLocal,
	)
	require.NoError(t, err)

	return &apiutil.Path{
		Family:         bgppacket.RF_IPv6_UC,
		Nlri:           nlri,
		PeerASN:        4200000001,
		PeerID:         netip.MustParseAddr("192.0.2.10"),
		PeerAddress:    linkLocal.WithZone("eth0"),
		IsFromExternal: true,
		Attrs: []bgppacket.PathAttributeInterface{
			bgppacket.NewPathAttributeOrigin(0),
			bgppacket.NewPathAttributeAsPath([]bgppacket.AsPathParamInterface{
				bgppacket.NewAs4PathParam(2, []uint32{4200000001}),
			}),
			mpReach,
		},
	}
}

// TestLearnedRouteNextHopOrder asserts that the next-hops of an ECMP route come out in a canonical
// order: GoBGP doesn't promise a stable path order, and the kernel keeps a multipath route's
// next-hops as installed, so an unstable order would rewrite the route on every reconcile.
func TestLearnedRouteNextHopOrder(t *testing.T) {
	ctx := t.Context()
	server := startBGPImportTestServer(t, ctx, 65000, "192.0.2.1", -1, true)

	prefix := netip.MustParsePrefix("203.0.113.0/24")
	high := netip.MustParseAddr("192.0.2.20")
	low := netip.MustParseAddr("192.0.2.10")

	// GoBGP breaks the best-path tie on the peer address, so a peer order opposite to the next-hop
	// order is what the sorting has to undo
	responses, err := server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{
		newBGPNumberedTestPath(t, prefix, netip.MustParseAddr("192.0.2.5"), high),
		newBGPNumberedTestPath(t, prefix, netip.MustParseAddr("192.0.2.6"), low),
	}})
	require.NoError(t, err)
	require.Len(t, responses, 2)

	for _, response := range responses {
		require.NoError(t, response.Error)
	}

	instance := internalbgp.NewInstance()
	internalbgp.SetInstanceServerForTest(instance, server)
	instance.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{InstallRoutes: true})

	assert.Equal(
		t,
		[]resourcenetwork.RouteNextHop{{Gateway: low}, {Gateway: high}},
		instance.Snapshot(ctx).Learned[prefix],
		"ECMP next-hops must be reported in a canonical order",
	)

	instance.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{InstallRoutes: true, MaxPaths: 1})

	assert.Equal(
		t,
		[]resourcenetwork.RouteNextHop{{Gateway: low}},
		instance.Snapshot(ctx).Learned[prefix],
		"maxPaths must cap the installed ECMP next-hops",
	)

	instance.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{InstallRoutes: true, MaxPaths: 2})

	assert.Equal(
		t,
		[]resourcenetwork.RouteNextHop{{Gateway: low}, {Gateway: high}},
		instance.Snapshot(ctx).Learned[prefix],
		"maxPaths equal to the next-hop count must keep all of them",
	)
}

// TestLearnedRouteDuplicateNextHops asserts that best paths sharing a next-hop (e.g. the same route
// reflected by two route reflectors with the next-hop unchanged) collapse into a single next-hop: the
// kernel rejects a duplicate IPv6 next-hop and double-weights a duplicate IPv4 one.
func TestLearnedRouteDuplicateNextHops(t *testing.T) {
	ctx := t.Context()
	server := startBGPImportTestServer(t, ctx, 65000, "192.0.2.1", -1, true)

	prefix := netip.MustParsePrefix("203.0.113.0/24")
	gateway := netip.MustParseAddr("192.0.2.10")

	responses, err := server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{
		newBGPNumberedTestPath(t, prefix, netip.MustParseAddr("192.0.2.5"), gateway),
		newBGPNumberedTestPath(t, prefix, netip.MustParseAddr("192.0.2.6"), gateway),
	}})
	require.NoError(t, err)
	require.Len(t, responses, 2)

	for _, response := range responses {
		require.NoError(t, response.Error)
	}

	instance := internalbgp.NewInstance()
	internalbgp.SetInstanceServerForTest(instance, server)
	instance.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{InstallRoutes: true})

	assert.Equal(
		t,
		[]resourcenetwork.RouteNextHop{{Gateway: gateway}},
		instance.Snapshot(ctx).Learned[prefix],
		"best paths sharing a next-hop must collapse into one",
	)
}

func TestLearnedNexthop(t *testing.T) {
	t.Parallel()

	nlri, err := bgppacket.NewIPAddrPrefix(netip.MustParsePrefix("2001:db8:3::/64"))
	require.NoError(t, err)

	ipv6Path := func(peer netip.Addr, nexthops ...netip.Addr) *apiutil.Path {
		mpReach, mpErr := bgppacket.NewPathAttributeMpReachNLRI(bgppacket.RF_IPv6_UC, []bgppacket.PathNLRI{{NLRI: nlri}}, nexthops...)
		require.NoError(t, mpErr)

		return &apiutil.Path{
			PeerAddress: peer,
			Attrs:       []bgppacket.PathAttributeInterface{bgppacket.NewPathAttributeOrigin(0), mpReach},
		}
	}

	ipv4Nexthop, err := bgppacket.NewPathAttributeNextHop(netip.MustParseAddr("192.0.2.10"))
	require.NoError(t, err)

	unnumbered := netip.MustParseAddr("fe80::2")
	peerIfaces := map[netip.Addr]string{unnumbered: "eth0"}

	for _, test := range []struct {
		path     *apiutil.Path
		expected resourcenetwork.RouteNextHop
		ok       bool
		name     string
	}{
		{
			name: "IPv4 next-hop",
			path: &apiutil.Path{
				PeerAddress: netip.MustParseAddr("192.0.2.10"),
				Attrs:       []bgppacket.PathAttributeInterface{bgppacket.NewPathAttributeOrigin(0), ipv4Nexthop},
			},
			expected: resourcenetwork.RouteNextHop{Gateway: netip.MustParseAddr("192.0.2.10")},
			ok:       true,
		},
		{
			name:     "unnumbered peer link-local next-hop",
			path:     ipv6Path(unnumbered, netip.MustParseAddr("2001:db8:9::2"), unnumbered),
			expected: resourcenetwork.RouteNextHop{Gateway: unnumbered, OutLinkName: "eth0"},
			ok:       true,
		},
		{
			name:     "numbered peer falls back to the global next-hop",
			path:     ipv6Path(netip.MustParseAddr("2001:db8:9::2"), netip.MustParseAddr("2001:db8:9::2"), netip.MustParseAddr("fe80::3")),
			expected: resourcenetwork.RouteNextHop{Gateway: netip.MustParseAddr("2001:db8:9::2")},
			ok:       true,
		},
		{
			name: "link-local next-hop only, unknown link",
			path: ipv6Path(netip.MustParseAddr("2001:db8:9::2"), netip.MustParseAddr("fe80::3")),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			nh, ok := internalbgp.LearnedNexthop(test.path, peerIfaces)

			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.expected, nh)
		})
	}
}
