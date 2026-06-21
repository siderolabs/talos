// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/jsimonetti/rtnetlink/v2"
	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// BGPController runs an embedded GoBGP speaker driven by the BGPPeerConfig resource.
//
// It originates the addresses of the configured links as host routes, installs the routes it
// learns from its neighbors as network.RouteSpec resources, and exposes peer state as
// network.BGPPeerStatus resources.
type BGPController struct {
	server     *gobgpsrv.BgpServer
	serverKey  string
	originated map[netip.Prefix]struct{}
	// peers maps an added peer's gobgp address to a hash of its configuration, so peers can be
	// reconciled incrementally (added/removed/updated) without restarting the whole server.
	peers map[string]string
	// peerIfaces maps a resolved unnumbered peer's link-local address to its link, used to set
	// the egress link on routes learned with a link-local next-hop.
	peerIfaces map[netip.Addr]string

	reconcileCh chan struct{}
}

// Name implements controller.Controller interface.
func (ctrl *BGPController) Name() string {
	return "network.BGPController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BGPController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.BGPPeerConfigType,
			ID:        optional.Some(network.BGPPeerConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.AddressStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *BGPController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.RouteSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.BGPPeerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *BGPController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.reconcileCh = make(chan struct{}, 1)
	ctrl.originated = map[netip.Prefix]struct{}{}
	ctrl.peers = map[string]string{}
	ctrl.peerIfaces = map[netip.Addr]string{}

	defer ctrl.stopServer()

	// unnumbered peers are discovered from the kernel neighbor table (populated by Router Advertisements /
	// NDP), which is not a COSI input — so watch rtnetlink neighbor events and reconcile the instant a
	// peer's link-local appears, via the existing r.EventCh() arm below (no polling latency).
	neighWatcher, err := watch.NewRtNetlink(trigger.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_NEIGH)
	if err != nil {
		return fmt.Errorf("error starting neighbor watch: %w", err)
	}

	defer neighWatcher.Done()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ctrl.reconcileCh:
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

// signal triggers a reconcile from a gobgp watch callback (non-blocking).
func (ctrl *BGPController) signal() {
	select {
	case ctrl.reconcileCh <- struct{}{}:
	default:
	}
}

func (ctrl *BGPController) stopServer() {
	if ctrl.server != nil {
		ctrl.server.Stop()
		ctrl.server = nil
	}

	ctrl.serverKey = ""
	ctrl.originated = map[netip.Prefix]struct{}{}
	ctrl.peers = map[string]string{}
}

//nolint:gocyclo,cyclop
func (ctrl *BGPController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	configResource, err := safe.ReaderGetByID[*network.BGPPeerConfig](ctx, r, network.BGPPeerConfigID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting BGP peer config: %w", err)
	}

	var bgpConfig *network.BGPPeerConfigSpec
	if configResource != nil {
		bgpConfig = configResource.TypedSpec()
	}

	if bgpConfig == nil {
		ctrl.stopServer()

		return ctrl.writeOutputs(ctx, r, nil, netip.Addr{}, nil)
	}

	advertised := ctrl.advertisedPrefixes(ctx, r, bgpConfig)

	routerID := ctrl.routerID(bgpConfig, advertised)
	if !routerID.IsValid() {
		logger.Warn("BGP router-id could not be determined, skipping BGP startup")

		ctrl.stopServer()

		return ctrl.writeOutputs(ctx, r, nil, netip.Addr{}, nil)
	}

	if err = ctrl.ensureServer(ctx, logger, bgpConfig, routerID); err != nil {
		return fmt.Errorf("error configuring BGP server: %w", err)
	}

	if err = ctrl.reconcileOriginated(advertised); err != nil {
		return fmt.Errorf("error originating BGP routes: %w", err)
	}

	learned := ctrl.listLearnedRoutes(advertised, ctrl.peerIfaces)
	peers := ctrl.listPeers(ctx, bgpConfig.LocalASN)

	return ctrl.writeOutputs(ctx, r, learned, bgpConfig.RouteSource, peers)
}

// ensureServer (re)creates the gobgp server when the server-level (global) configuration changes, then
// reconciles the peer set incrementally — so a newly-discovered or removed neighbor never restarts the
// server (and never bounces the other established sessions).
func (ctrl *BGPController) ensureServer(ctx context.Context, logger *zap.Logger, bgpConfig *network.BGPPeerConfigSpec, routerID netip.Addr) error {
	// resolve neighbors (unnumbered peers resolve their link-local from the kernel neighbor table,
	// populated via Router Advertisements); skip peers not yet discovered (reconciled on the next event).
	ctrl.peerIfaces = map[netip.Addr]string{}

	var resolved []internalbgp.Peer

	for _, neighbor := range bgpConfig.Neighbors {
		peer, ok := resolveNeighborPeer(neighbor, logger)
		if !ok {
			logger.Debug("unnumbered BGP peer not yet discovered, will retry", zap.String("link", neighbor.Link))

			continue
		}

		resolved = append(resolved, peer)

		if peer.LinkLocal.IsValid() {
			ctrl.peerIfaces[peer.LinkLocal] = peer.Link
		}
	}

	key := internalbgp.ServerKey(bgpConfig.LocalASN, routerID, bgpConfig.Multipath, bgpConfig.MaxPaths)

	if ctrl.server == nil || ctrl.serverKey != key {
		ctrl.stopServer()

		// route gobgp's logs into the controller's zap logger (gobgp's LoggerOption requires an *slog.Logger);
		// the level var gates gobgp at warn+ to keep it quiet, zap applies the final filtering.
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelWarn)

		srv := gobgpsrv.NewBgpServer(gobgpsrv.LoggerOption(slog.New(zapslog.NewHandler(logger.Core())), lvl))

		go srv.Serve()

		global := &gobgpapi.Global{
			Asn:              bgpConfig.LocalASN,
			RouterId:         routerID.String(),
			ListenPort:       constants.BGPDefaultPort,
			UseMultiplePaths: bgpConfig.Multipath,
		}

		if err := srv.StartBgp(ctx, &gobgpapi.StartBgpRequest{Global: global}); err != nil {
			srv.Stop()

			return fmt.Errorf("error starting BGP: %w", err)
		}

		if err := srv.WatchEvent(ctx, gobgpsrv.WatchEventMessageCallbacks{
			OnBestPath: func([]*apiutil.Path, time.Time) {
				ctrl.signal()
			},
			OnPeerUpdate: func(*apiutil.WatchEventMessage_PeerEvent, time.Time) {
				ctrl.signal()
			},
		}, gobgpsrv.WatchBestPath(true), gobgpsrv.WatchPeer()); err != nil {
			srv.Stop()

			return fmt.Errorf("error watching BGP events: %w", err)
		}

		ctrl.server = srv
		ctrl.serverKey = key
		ctrl.originated = map[netip.Prefix]struct{}{}
		ctrl.peers = map[string]string{}

		logger.Info("started embedded BGP speaker", zap.Uint32("asn", bgpConfig.LocalASN), zap.Stringer("router_id", routerID))
	}

	return ctrl.reconcilePeers(ctx, bgpConfig, resolved)
}

// reconcilePeers diffs the resolved neighbor set against the peers currently configured on the running
// gobgp server, adding new (or changed) peers and removing stale ones — without restarting the server.
func (ctrl *BGPController) reconcilePeers(ctx context.Context, bgpConfig *network.BGPPeerConfigSpec, resolved []internalbgp.Peer) error {
	desired := make(map[string]string, len(resolved))

	for _, peer := range resolved {
		desired[peer.Address] = internalbgp.PeerKey(peer)
	}

	// remove peers that are gone or whose configuration changed (re-added below).
	for address, hash := range ctrl.peers {
		if desired[address] == hash {
			continue
		}

		if err := ctrl.server.DeletePeer(ctx, &gobgpapi.DeletePeerRequest{Address: address}); err != nil {
			return fmt.Errorf("error deleting BGP peer: %w", err)
		}

		delete(ctrl.peers, address)
	}

	// add new (or changed) peers.
	for _, peer := range resolved {
		if _, ok := ctrl.peers[peer.Address]; ok {
			continue
		}

		if err := ctrl.server.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: internalbgp.BuildPeer(peer, bgpConfig.Multipath)}); err != nil {
			return fmt.Errorf("error adding BGP peer: %w", err)
		}

		ctrl.peers[peer.Address] = desired[peer.Address]
	}

	return nil
}

// reconcileOriginated diffs the desired advertised prefixes against what is currently originated.
func (ctrl *BGPController) reconcileOriginated(advertised []netip.Prefix) error {
	desired := make(map[netip.Prefix]struct{}, len(advertised))

	for _, prefix := range advertised {
		desired[prefix] = struct{}{}

		if _, ok := ctrl.originated[prefix]; ok {
			continue
		}

		path, err := internalbgp.BuildOriginatedPath(prefix)
		if err != nil {
			return err
		}

		if _, err = ctrl.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}}); err != nil {
			return fmt.Errorf("error adding path %s: %w", prefix, err)
		}

		ctrl.originated[prefix] = struct{}{}
	}

	for prefix := range ctrl.originated {
		if _, ok := desired[prefix]; ok {
			continue
		}

		path, err := internalbgp.BuildOriginatedPath(prefix)
		if err != nil {
			return err
		}

		if err = ctrl.server.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}}); err != nil {
			return fmt.Errorf("error deleting path %s: %w", prefix, err)
		}

		delete(ctrl.originated, prefix)
	}

	return nil
}

// listLearnedRoutes builds the set of best-path routes learned from peers, keyed by destination.
//
// Locally originated prefixes are excluded.
//
//nolint:gocyclo
func (ctrl *BGPController) listLearnedRoutes(advertised []netip.Prefix, peerIfaces map[netip.Addr]string) map[netip.Prefix][]network.RouteNextHop {
	learned := map[netip.Prefix][]network.RouteNextHop{}

	advertisedSet := make(map[netip.Prefix]struct{}, len(advertised))
	for _, prefix := range advertised {
		advertisedSet[prefix] = struct{}{}
	}

	for _, family := range []bgppacket.Family{bgppacket.RF_IPv4_UC, bgppacket.RF_IPv6_UC} {
		err := ctrl.server.ListPath(apiutil.ListPathRequest{
			TableType: gobgpapi.TableType_TABLE_TYPE_GLOBAL,
			Family:    family,
		}, func(prefix bgppacket.NLRI, paths []*apiutil.Path) {
			dst, parseErr := netip.ParsePrefix(prefix.String())
			if parseErr != nil {
				return
			}

			if _, ok := advertisedSet[dst]; ok {
				return
			}

			for _, path := range paths {
				if !path.Best || path.Withdrawal {
					continue
				}

				nexthop := internalbgp.PathNexthop(path)
				if !nexthop.IsValid() || nexthop.IsUnspecified() {
					continue
				}

				nh := network.RouteNextHop{Gateway: nexthop}

				// a link-local next-hop (unnumbered/RFC 8950) needs an explicit egress interface,
				// resolved from the peer it was learned from. peerIfaces is keyed by the bare
				// link-local, while gobgp reports the peer address with the interface zone.
				if nexthop.IsLinkLocalUnicast() {
					nh.OutLinkName = peerIfaces[path.PeerAddress.WithZone("")]
				}

				learned[dst] = append(learned[dst], nh)
			}
		})
		if err != nil {
			// best-effort: ListPath may fail for a not-yet-active family
			continue
		}
	}

	return learned
}

// listPeers queries gobgp for the current peer state.
func (ctrl *BGPController) listPeers(ctx context.Context, localASN uint32) []network.BGPPeerStatusSpec {
	var peers []network.BGPPeerStatusSpec

	if err := ctrl.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(p *gobgpapi.Peer) {
		peers = append(peers, internalbgp.PeerStatus(p, localASN))
	}); err != nil {
		return nil
	}

	return peers
}

// writeOutputs reconciles RouteSpec and BGPPeerStatus resources owned by this controller.
func (ctrl *BGPController) writeOutputs(ctx context.Context, r controller.Runtime, learned map[netip.Prefix][]network.RouteNextHop, source netip.Addr, peers []network.BGPPeerStatusSpec) error {
	r.StartTrackingOutputs()

	for prefix, nexthops := range learned {
		spec := internalbgp.RouteSpec(prefix, nexthops, source)

		id := "bgp/" + network.RouteID(spec.Table, spec.Family, spec.Destination, spec.Gateway, spec.Priority, spec.OutLinkName)

		if err := safe.WriterModify(ctx, r, network.NewRouteSpec(network.ConfigNamespaceName, id), func(route *network.RouteSpec) error {
			*route.TypedSpec() = spec

			return nil
		}); err != nil {
			return fmt.Errorf("error writing route spec: %w", err)
		}
	}

	for _, peer := range peers {
		if err := safe.WriterModify(ctx, r, network.NewBGPPeerStatus(network.NamespaceName, peer.Peer), func(status *network.BGPPeerStatus) error {
			*status.TypedSpec() = peer

			return nil
		}); err != nil {
			return fmt.Errorf("error writing BGP peer status: %w", err)
		}
	}

	if err := r.CleanupOutputs(
		ctx,
		resource.NewMetadata(network.ConfigNamespaceName, network.RouteSpecType, "", resource.VersionUndefined),
		resource.NewMetadata(network.NamespaceName, network.BGPPeerStatusType, "", resource.VersionUndefined),
	); err != nil {
		return fmt.Errorf("error cleaning up outputs: %w", err)
	}

	return nil
}

// advertisedPrefixes collects host prefixes (/32, /128) of the configured advertised links.
func (ctrl *BGPController) advertisedPrefixes(ctx context.Context, r controller.Runtime, bgpConfig *network.BGPPeerConfigSpec) []netip.Prefix {
	links := make(map[string]struct{}, len(bgpConfig.AdvertiseLinks))
	for _, link := range bgpConfig.AdvertiseLinks {
		links[link] = struct{}{}
	}

	if len(links) == 0 {
		return nil
	}

	addresses, err := safe.ReaderListAll[*network.AddressStatus](ctx, r)
	if err != nil {
		return nil
	}

	var prefixes []netip.Prefix

	for address := range addresses.All() {
		spec := address.TypedSpec()

		if _, ok := links[spec.LinkName]; !ok {
			continue
		}

		addr := spec.Address.Addr()

		if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
			continue
		}

		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}

	return prefixes
}

// routerID picks the BGP router-id: configured value, or the first advertised IPv4 address.
func (ctrl *BGPController) routerID(bgpConfig *network.BGPPeerConfigSpec, advertised []netip.Prefix) netip.Addr {
	if id := bgpConfig.RouterID; id.IsValid() {
		return id
	}

	for _, prefix := range advertised {
		if prefix.Addr().Is4() {
			return prefix.Addr()
		}
	}

	return netip.Addr{}
}

// resolveNeighborPeer resolves a neighbor's BGP address. Numbered peers use the configured address;
// unnumbered peers resolve their single link-local neighbor from the kernel neighbor table (populated
// via Router Advertisements) and use a zoned address (fe80::x%iface). Returns false if an unnumbered
// peer is not yet discovered.
func resolveNeighborPeer(neighbor network.BGPNeighborConfigSpec, logger *zap.Logger) (internalbgp.Peer, bool) {
	peer := internalbgp.Peer{Config: neighbor}

	if addr := neighbor.Address; addr.IsValid() {
		peer.Address = addr.String()

		return peer, true
	}

	iface := neighbor.Link
	if iface == "" {
		return internalbgp.Peer{}, false
	}

	lla, realName, ok := linkLocalNeighbor(iface, logger)
	if !ok {
		return internalbgp.Peer{}, false
	}

	peer.Address = lla.String() + "%" + realName
	peer.LinkLocal = lla
	peer.Link = realName

	return peer, true
}

// linkLocalNeighbor returns the single IPv6 link-local neighbor on the interface (the unnumbered peer),
// along with the interface's real kernel name (the configured name may be a Talos alias/altname, which
// the kernel cannot use as a scope zone). It returns false unless exactly one such neighbor is present
// (point-to-point assumption).
//
//nolint:gocyclo,cyclop
func linkLocalNeighbor(iface string, logger *zap.Logger) (netip.Addr, string, bool) {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return netip.Addr{}, "", false
	}

	defer conn.Close() //nolint:errcheck

	links, err := conn.Link.List()
	if err != nil {
		return netip.Addr{}, "", false
	}

	// The intermediate config normally carries a canonical link name, but LinkStatus may lag rtnetlink
	// during link changes. Resolve aliases and altnames here as a kernel-side fallback.
	index := resolveLinkName(links, iface)
	if index == 0 {
		return netip.Addr{}, "", false
	}

	var realName string

	for _, link := range links {
		if link.Index == index {
			realName = link.Attributes.Name

			break
		}
	}

	// the interface's own link-local addresses must never be treated as a neighbor (the link may
	// loop our own frames back, e.g. some QEMU netdevs).
	ownAddrs := map[netip.Addr]struct{}{}

	if addrs, addrErr := conn.Address.List(); addrErr == nil {
		for _, a := range addrs {
			if a.Index != index || a.Attributes == nil {
				continue
			}

			if addr, ok := netip.AddrFromSlice(a.Attributes.Address); ok {
				ownAddrs[addr.Unmap()] = struct{}{}
			}
		}
	}

	neighbors, err := conn.Neigh.List()
	if err != nil {
		return netip.Addr{}, "", false
	}

	var candidates []netip.Addr

	for _, n := range neighbors {
		if n.Index != index || n.Attributes == nil {
			continue
		}

		if n.State&(unix.NUD_FAILED|unix.NUD_INCOMPLETE) != 0 {
			continue
		}

		// the BGP peer announces itself via Router Advertisements (NTF_ROUTER); other link-local
		// neighbors on a shared L2 (e.g. tc-redirect-tap veths) are not routers and must be excluded.
		if n.Flags&unix.NTF_ROUTER == 0 {
			continue
		}

		addr, ok := netip.AddrFromSlice(n.Attributes.Address)
		if !ok || !addr.IsLinkLocalUnicast() {
			continue
		}

		if _, self := ownAddrs[addr.Unmap()]; self {
			continue
		}

		candidates = append(candidates, addr)
	}

	if len(candidates) != 1 {
		logger.Debug("unnumbered peer resolution needs exactly one link-local neighbor",
			zap.String("interface", iface),
			zap.Int("count", len(candidates)),
			zap.Strings("candidates", xslices.Map(candidates, netip.Addr.String)),
		)

		return netip.Addr{}, "", false
	}

	return candidates[0], realName, true
}
