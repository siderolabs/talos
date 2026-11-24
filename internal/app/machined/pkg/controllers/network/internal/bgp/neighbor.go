// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"fmt"
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
	load      func() ([]rtnetlink.NeighMessage, error)
	neighbors []rtnetlink.NeighMessage
	loadErr   error
}

// NewNeighborResolver creates a neighbor resolver which loads the neighbor table with load on first use.
func NewNeighborResolver(load func() ([]rtnetlink.NeighMessage, error)) *NeighborResolver {
	return &NeighborResolver{load: load}
}

// ResolvePeers resolves all currently discoverable peers and their link-local interface mapping.
//
// If the neighbor table can't be read, unnumbered peers keep their link-local address from current (the
// running unnumbered peers by link), as an empty table would remove them, and the read error is returned
// together with the resolved peers.
func (resolver *NeighborResolver) ResolvePeers(
	config network.BGPInstanceConfigSpec,
	runtimeState *RuntimeState,
	current map[string]netip.Addr,
	logger *zap.Logger,
) ([]Peer, map[netip.Addr]string, error) {
	peerIfaces := map[netip.Addr]string{}
	resolved := make([]Peer, 0, len(config.Neighbors))

	for _, neighbor := range config.Neighbors {
		if neighbor.Prefix.IsValid() {
			continue
		}

		peer, ok := resolver.resolvePeer(neighbor, runtimeState, current, logger)
		if !ok {
			logger.Debug("unnumbered BGP peer not yet discovered, will retry", zap.String("link", neighbor.Link))

			continue
		}

		if peer.LinkLocal.IsValid() {
			warnAuthPasswordConflict(resolved, peer, logger)

			peerIfaces[peer.LinkLocal] = peer.Link
		}

		if config.VRF != "" {
			peer.BindInterface = config.VRF
		} else if peer.Link != "" {
			peer.BindInterface = peer.Link
		}

		resolved = append(resolved, peer)
	}

	return resolved, peerIfaces, resolver.loadErr
}

// warnAuthPasswordConflict reports an unnumbered peer whose link-local address is shared with an already
// resolved peer on another link, but with a different authPassword.
//
// The kernel matches TCP MD5 keys by peer address (scoped to the VRF, if any, but not to the link), so GoBGP
// installs a single key for the address on the listener, and only one of the two sessions can authenticate.
func warnAuthPasswordConflict(resolved []Peer, peer Peer, logger *zap.Logger) {
	for _, other := range resolved {
		if other.LinkLocal != peer.LinkLocal || other.Link == peer.Link {
			continue
		}

		if other.Config.AuthPassword == peer.Config.AuthPassword {
			continue
		}

		logger.Warn(
			"unnumbered BGP peers share a link-local address but use different authPassword values, only one session can authenticate",
			zap.Stringer("peer", peer.LinkLocal),
			zap.String("link", peer.Link),
			zap.String("other_link", other.Link),
		)

		return
	}
}

func (resolver *NeighborResolver) resolvePeer(
	neighbor network.BGPNeighborConfigSpec,
	runtimeState *RuntimeState,
	current map[string]netip.Addr,
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
		resolver.neighbors, resolver.loadErr = resolver.load()
	})

	if resolver.loadErr == nil {
		return ResolvePeer(neighbor, runtimeState, resolver.neighbors, logger)
	}

	lla, ok := current[neighbor.Link]
	if !ok {
		return Peer{}, false
	}

	return unnumberedPeer(neighbor, lla), true
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

	return unnumberedPeer(neighbor, lla), true
}

func unnumberedPeer(neighbor network.BGPNeighborConfigSpec, lla netip.Addr) Peer {
	return Peer{
		Config:    neighbor,
		Address:   lla.String() + "%" + neighbor.Link,
		LinkLocal: lla,
		Link:      neighbor.Link,
	}
}

// ReadNeighbors lists the kernel neighbor table.
func ReadNeighbors() ([]rtnetlink.NeighMessage, error) {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return nil, fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	neighbors, err := conn.Neigh.List()
	if err != nil {
		return nil, fmt.Errorf("error listing neighbors: %w", err)
	}

	return neighbors, nil
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
