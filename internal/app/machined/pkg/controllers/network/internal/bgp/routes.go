// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"context"
	"net/netip"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Snapshot is the current output state of a running BGP instance.
type Snapshot struct {
	Table        nethelpers.RoutingTable
	Source       netip.Addr
	Learned      map[netip.Prefix][]network.RouteNextHop
	PeerStatuses []network.BGPPeerStatusSpec
}

// Snapshot returns learned routes and peer state from the running GoBGP server.
func (instance *Instance) Snapshot(ctx context.Context) Snapshot {
	var learned map[netip.Prefix][]network.RouteNextHop
	if instance.installRoutes {
		learned = instance.learnedRoutes()
	}

	return Snapshot{
		Table:        instance.table,
		Source:       instance.source,
		Learned:      learned,
		PeerStatuses: instance.peerStatuses(ctx),
	}
}

func (instance *Instance) learnedRoutes() map[netip.Prefix][]network.RouteNextHop {
	learned := map[netip.Prefix][]network.RouteNextHop{}

	localSet := make(map[netip.Prefix]struct{}, len(instance.connected)+len(instance.imported))
	for _, prefix := range instance.connected {
		localSet[prefix] = struct{}{}
	}

	for prefix := range instance.imported {
		localSet[prefix] = struct{}{}
	}

	for _, family := range []bgppacket.Family{bgppacket.RF_IPv4_UC, bgppacket.RF_IPv6_UC} {
		// Best-effort: ListPath may fail for a not-yet-active family.
		instance.listPaths(family, func(prefix netip.Prefix, paths []*apiutil.Path) { //nolint:errcheck
			if _, local := localSet[prefix]; local {
				return
			}

			if nexthops := LearnedNexthops(paths, instance.peerIfaces, instance.maxPaths); len(nexthops) > 0 {
				learned[prefix] = nexthops
			}
		})
	}

	return learned
}

// listPaths calls fn with the paths of each destination of a family in the global RIB.
func (instance *Instance) listPaths(family bgppacket.Family, fn func(netip.Prefix, []*apiutil.Path)) error {
	return instance.server.ListPath(apiutil.ListPathRequest{
		TableType: gobgpapi.TableType_TABLE_TYPE_GLOBAL,
		Family:    family,
	}, func(nlri bgppacket.NLRI, paths []*apiutil.Path) {
		if prefix, err := netip.ParsePrefix(nlri.String()); err == nil {
			fn(prefix, paths)
		}
	})
}

// LearnedNexthops returns the next-hops to install for the best paths of a destination.
func LearnedNexthops(paths []*apiutil.Path, peerIfaces map[netip.Addr]string, maxPaths uint8) []network.RouteNextHop {
	var nexthops []network.RouteNextHop

	for _, path := range paths {
		if !path.Best || path.Withdrawal {
			continue
		}

		if nh, ok := LearnedNexthop(path, peerIfaces); ok {
			nexthops = append(nexthops, nh)
		}
	}

	if len(nexthops) == 0 {
		return nil
	}

	// GoBGP yields the paths of a destination in its own order, which is not guaranteed to be stable
	// across calls, and several best paths may share a next-hop (e.g. the same route reflected by two
	// route reflectors): canonicalize the ECMP set so that the RouteSpec is stable across reconciles.
	// The canonical order also makes the maxPaths cut stable.
	nexthops = network.NormalizeNextHops(nexthops)

	if maxPaths > 0 && len(nexthops) > int(maxPaths) {
		nexthops = nexthops[:maxPaths]
	}

	return nexthops
}

// LearnedNexthop returns the next-hop to install for a learned path.
//
// A link-local gateway is only meaningful together with the egress link, which is known for unnumbered
// peers only. Paths from other peers fall back to the global next-hop (RFC 2545 sends both), and are
// dropped without one: the kernel rejects a link-local gateway without a device.
func LearnedNexthop(path *apiutil.Path, peerIfaces map[netip.Addr]string) (network.RouteNextHop, bool) {
	nexthop := PathNexthop(path)
	if !nexthop.IsValid() || nexthop.IsUnspecified() {
		return network.RouteNextHop{}, false
	}

	if !nexthop.IsLinkLocalUnicast() {
		return network.RouteNextHop{Gateway: nexthop}, true
	}

	if link := peerIfaces[path.PeerAddress.WithZone("")]; link != "" {
		return network.RouteNextHop{Gateway: nexthop, OutLinkName: link}, true
	}

	for _, attr := range path.Attrs {
		mpReach, ok := attr.(*bgppacket.PathAttributeMpReachNLRI)
		if ok && mpReach.Nexthop.IsValid() && !mpReach.Nexthop.IsUnspecified() && !mpReach.Nexthop.IsLinkLocalUnicast() {
			return network.RouteNextHop{Gateway: mpReach.Nexthop}, true
		}
	}

	return network.RouteNextHop{}, false
}

func (instance *Instance) peerStatuses(ctx context.Context) []network.BGPPeerStatusSpec {
	var peers []network.BGPPeerStatusSpec

	if err := instance.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(peer *gobgpapi.Peer) {
		peers = append(peers, PeerStatus(peer, instance.localASN))
	}); err != nil {
		return nil
	}

	return peers
}
