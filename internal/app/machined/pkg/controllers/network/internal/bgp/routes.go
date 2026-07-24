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

//nolint:gocyclo
func (instance *Instance) learnedRoutes() map[netip.Prefix][]network.RouteNextHop {
	learned := map[netip.Prefix][]network.RouteNextHop{}

	localSet := make(map[netip.Prefix]struct{}, len(instance.advertised)+len(instance.imported))
	for _, prefix := range instance.advertised {
		localSet[prefix] = struct{}{}
	}

	for prefix := range instance.imported {
		localSet[prefix] = struct{}{}
	}

	for _, family := range []bgppacket.Family{bgppacket.RF_IPv4_UC, bgppacket.RF_IPv6_UC} {
		err := instance.server.ListPath(apiutil.ListPathRequest{
			TableType: gobgpapi.TableType_TABLE_TYPE_GLOBAL,
			Family:    family,
		}, func(prefix bgppacket.NLRI, paths []*apiutil.Path) {
			dst, parseErr := netip.ParsePrefix(prefix.String())
			if parseErr != nil {
				return
			}

			if _, ok := localSet[dst]; ok {
				return
			}

			for _, path := range paths {
				if !path.Best || path.Withdrawal {
					continue
				}

				nexthop := PathNexthop(path)
				if !nexthop.IsValid() || nexthop.IsUnspecified() {
					continue
				}

				nh := network.RouteNextHop{Gateway: nexthop}

				if nexthop.IsLinkLocalUnicast() {
					nh.OutLinkName = instance.peerIfaces[path.PeerAddress.WithZone("")]
				}

				learned[dst] = append(learned[dst], nh)
			}
		})
		if err != nil {
			// Best-effort: ListPath may fail for a not-yet-active family.
			continue
		}
	}

	return learned
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
