// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"net/netip"
	"sync"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NeighborResolver resolves configured BGP peers against one lazily loaded rtnetlink snapshot.
type NeighborResolver struct {
	once      sync.Once
	load      func() []rtnetlink.NeighMessage
	neighbors []rtnetlink.NeighMessage
}

// NewNeighborResolver creates a neighbor resolver.
func NewNeighborResolver() *NeighborResolver {
	return &NeighborResolver{load: readNeighbors}
}

// ResolvePeers resolves all currently discoverable peers and their link-local interface mapping.
func (resolver *NeighborResolver) ResolvePeers(
	config network.BGPInstanceConfigSpec,
	runtimeState *RuntimeState,
	logger *zap.Logger,
) ([]Peer, map[netip.Addr]string) {
	peerIfaces := map[netip.Addr]string{}
	resolved := make([]Peer, 0, len(config.Neighbors))

	for _, neighbor := range config.Neighbors {
		peer, ok := resolver.resolvePeer(neighbor, runtimeState, logger)
		if !ok {
			logger.Debug("unnumbered BGP peer not yet discovered, will retry", zap.String("link", neighbor.Link))

			continue
		}

		if peer.LinkLocal.IsValid() {
			peerIfaces[peer.LinkLocal] = peer.Link
		}

		if config.VRF != "" {
			peer.BindInterface = config.VRF
		} else if peer.Link != "" {
			peer.BindInterface = peer.Link
		}

		if neighbor.Address.IsValid() {
			peer.LocalAddress = runtimeState.numberedPeerSource(neighbor.Address, config.VRF)
		}

		resolved = append(resolved, peer)
	}

	return resolved, peerIfaces
}

func (resolver *NeighborResolver) resolvePeer(
	neighbor network.BGPNeighborConfigSpec,
	runtimeState *RuntimeState,
	logger *zap.Logger,
) (Peer, bool) {
	if addr := neighbor.Address; addr.IsValid() {
		return ResolvePeer(neighbor, runtimeState, nil, logger)
	}

	if neighbor.Link == "" {
		return Peer{}, false
	}

	if _, ok := runtimeState.LinkIndexByName(neighbor.Link); !ok {
		return Peer{}, false
	}

	resolver.once.Do(func() {
		resolver.neighbors = resolver.load()
	})

	return ResolvePeer(neighbor, runtimeState, resolver.neighbors, logger)
}

// ResolvePeer resolves one configured peer against a neighbor snapshot.
func ResolvePeer(
	neighbor network.BGPNeighborConfigSpec,
	runtimeState *RuntimeState,
	neighbors []rtnetlink.NeighMessage,
	logger *zap.Logger,
) (Peer, bool) {
	peer := Peer{Config: neighbor}

	if addr := neighbor.Address; addr.IsValid() {
		peer.Address = addr.String()

		return peer, true
	}

	if neighbor.Link == "" {
		return Peer{}, false
	}

	linkIndex, ok := runtimeState.LinkIndexByName(neighbor.Link)
	if !ok {
		return Peer{}, false
	}

	lla, ok := SelectLinkLocalNeighbor(
		neighbors,
		neighbor.Link,
		linkIndex,
		runtimeState.OwnAddresses(linkIndex),
		logger,
	)
	if !ok {
		return Peer{}, false
	}

	peer.Address = lla.String() + "%" + neighbor.Link
	peer.LinkLocal = lla
	peer.Link = neighbor.Link

	return peer, true
}

func readNeighbors() []rtnetlink.NeighMessage {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return nil
	}

	defer conn.Close() //nolint:errcheck

	neighbors, err := conn.Neigh.List()
	if err != nil {
		return nil
	}

	return neighbors
}

// SelectLinkLocalNeighbor returns the single IPv6 link-local router neighbor on the interface.
func SelectLinkLocalNeighbor(
	neighbors []rtnetlink.NeighMessage,
	iface string,
	index uint32,
	ownAddrs map[netip.Addr]struct{},
	logger *zap.Logger,
) (netip.Addr, bool) {
	var candidates []netip.Addr

	for _, neighbor := range neighbors {
		if neighbor.Index != index || neighbor.Attributes == nil {
			continue
		}

		if neighbor.State&(unix.NUD_FAILED|unix.NUD_INCOMPLETE) != 0 {
			continue
		}

		if neighbor.Flags&unix.NTF_ROUTER == 0 {
			continue
		}

		addr, ok := netip.AddrFromSlice(neighbor.Attributes.Address)
		if !ok || !addr.IsLinkLocalUnicast() {
			continue
		}

		if _, self := ownAddrs[addr.Unmap()]; self {
			continue
		}

		candidates = append(candidates, addr)
	}

	if len(candidates) != 1 {
		logger.Debug(
			"unnumbered peer resolution needs exactly one link-local neighbor",
			zap.String("interface", iface),
			zap.Int("count", len(candidates)),
			zap.Strings("candidates", xslices.Map(candidates, netip.Addr.String)),
		)

		return netip.Addr{}, false
	}

	return candidates[0], true
}
