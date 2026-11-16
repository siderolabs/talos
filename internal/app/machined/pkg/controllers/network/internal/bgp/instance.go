// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"context"
	"errors"
	"fmt"
	"log/slog" //nolint:loglinter // GoBGP's logger compatibility interface requires slog.
	"net/netip"
	"slices"
	"sync"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Instance owns the lifecycle and reconciled state of one GoBGP server.
type Instance struct {
	server        *gobgpsrv.BgpServer
	serverKey     string
	derivedID     netip.Addr
	pendingID     netip.Addr
	watchCancel   context.CancelFunc
	originated    map[netip.Prefix]struct{}
	imported      map[netip.Prefix]importedPath
	connected     []netip.Prefix
	imports       []network.BGPImportRouteSpec
	table         nethelpers.RoutingTable
	source        netip.Addr
	localASN      uint32
	installRoutes bool
	peers         map[string]string
	peerIfaces    map[netip.Addr]string
	unnumbered    map[string]netip.Addr
}

// NewInstance creates an initialized, stopped BGP instance.
func NewInstance() *Instance {
	return &Instance{
		originated: map[netip.Prefix]struct{}{},
		imported:   map[netip.Prefix]importedPath{},
		peers:      map[string]string{},
		peerIfaces: map[netip.Addr]string{},
		unnumbered: map[string]netip.Addr{},
	}
}

// Running reports whether the instance has an active GoBGP server.
func (instance *Instance) Running() bool {
	return instance.server != nil
}

// UnnumberedPeers returns the link-local address of each running unnumbered peer, by its link.
func (instance *Instance) UnnumberedPeers() map[string]netip.Addr {
	return instance.unnumbered
}

// Stop tears down the GoBGP server and resets all reconciled state.
func (instance *Instance) Stop() {
	if instance.watchCancel != nil {
		instance.watchCancel()
		instance.watchCancel = nil
	}

	if instance.server != nil {
		instance.server.Stop()
		instance.server = nil
	}

	instance.serverKey = ""
	instance.derivedID = netip.Addr{}
	instance.pendingID = netip.Addr{}

	instance.clearState()
}

func (instance *Instance) clearState() {
	instance.originated = map[netip.Prefix]struct{}{}
	instance.imported = map[netip.Prefix]importedPath{}
	instance.connected = nil
	instance.imports = nil
	instance.table = 0
	instance.source = netip.Addr{}
	instance.localASN = 0
	instance.installRoutes = false
	instance.peers = map[string]string{}
	instance.peerIfaces = map[netip.Addr]string{}
	instance.unnumbered = map[string]netip.Addr{}
}

// EnsureServer (re)creates the GoBGP server when server-level configuration changes, then
// reconciles its peer set incrementally.
func (instance *Instance) EnsureServer(
	ctx context.Context,
	logger *zap.Logger,
	config network.BGPInstanceConfigSpec,
	routerIDCandidates []netip.Addr,
	vrfIndex uint32,
	resolvedPeers []Peer,
	peerIfaces map[netip.Addr]string,
	listenPort int32,
	signal func(),
) error {
	routerID, key := instance.serverIdentity(ctx, logger, config, routerIDCandidates, resolvedPeers, vrfIndex, listenPort)

	if instance.server == nil || instance.serverKey != key {
		instance.Stop()

		// Route GoBGP's logs into the controller's zap logger. GoBGP's LoggerOption requires an
		// *slog.Logger; the level var gates GoBGP at warn+, and zap applies the final filtering.
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelWarn)

		srv := gobgpsrv.NewBgpServer(gobgpsrv.LoggerOption(slog.New(zapslog.NewHandler(logger.Core())), lvl))

		go srv.Serve()

		global := &gobgpapi.Global{
			Asn:              config.LocalASN,
			RouterId:         routerID.String(),
			ListenPort:       listenPort,
			ListenAddresses:  listenAddresses(),
			UseMultiplePaths: config.Multipath,
			BindToDevice:     config.VRF,
		}

		if err := srv.StartBgp(ctx, &gobgpapi.StartBgpRequest{Global: global}); err != nil {
			srv.Stop()

			return fmt.Errorf("error starting BGP: %w", err)
		}

		watchCtx, watchCancel := context.WithCancel(ctx)

		if err := srv.WatchEvent(watchCtx, gobgpsrv.WatchEventMessageCallbacks{
			OnBestPath: func([]*apiutil.Path, time.Time) {
				signal()
			},
			OnPeerUpdate: func(*apiutil.WatchEventMessage_PeerEvent, time.Time) {
				signal()
			},
		}, gobgpsrv.WatchBestPath(true), gobgpsrv.WatchPeer()); err != nil {
			watchCancel()
			srv.Stop()

			return fmt.Errorf("error watching BGP events: %w", err)
		}

		instance.server = srv
		instance.serverKey = key
		instance.watchCancel = watchCancel
		instance.originated = map[netip.Prefix]struct{}{}
		instance.peers = map[string]string{}

		logger.Info("started embedded BGP speaker", zap.Uint32("asn", config.LocalASN), zap.Stringer("router_id", routerID))
	}

	// a configured router ID may be dropped for the same derived one without a rebuild
	instance.derivedID = netip.Addr{}

	if !config.RouterID.IsValid() {
		instance.derivedID = routerID
	}

	instance.peerIfaces = peerIfaces
	instance.unnumbered = map[string]netip.Addr{}

	for _, peer := range resolvedPeers {
		if peer.LinkLocal.IsValid() {
			instance.unnumbered[peer.Link] = peer.LinkLocal
		}
	}

	return instance.reconcilePeers(ctx, config, resolvedPeers)
}

// serverIdentity returns the router ID and server key to run with.
//
// Like FRR, a derived router ID is kept while a session is established or opening, as a new one
// resets every session, unless the server is rebuilt anyway or the address has left the advertised
// links. derivedID is unset while the router ID is configured.
func (instance *Instance) serverIdentity(
	ctx context.Context,
	logger *zap.Logger,
	config network.BGPInstanceConfigSpec,
	routerIDCandidates []netip.Addr,
	resolvedPeers []Peer,
	vrfIndex uint32,
	listenPort int32,
) (netip.Addr, string) {
	routerID := selectRouterID(config.RouterID, routerIDCandidates)
	key := ServerKey(config.LocalASN, routerID, config.Multipath, config.MaxPaths, config.VRF, vrfIndex, config.VRFTable, listenPort)
	keptKey := ServerKey(config.LocalASN, instance.derivedID, config.Multipath, config.MaxPaths, config.VRF, vrfIndex, config.VRFTable, listenPort)

	keep := key != instance.serverKey && keptKey == instance.serverKey && !config.RouterID.IsValid() &&
		slices.Contains(routerIDCandidates, instance.derivedID) && instance.hasSession(ctx, resolvedPeers)

	if !keep {
		instance.pendingID = netip.Addr{}

		return routerID, key
	}

	if instance.pendingID != routerID {
		logger.Info("keeping the BGP router ID while a session is established or opening",
			zap.Stringer("router_id", instance.derivedID), zap.Stringer("derived_router_id", routerID))

		instance.pendingID = routerID
	}

	return instance.derivedID, keptKey
}

// hasSession reports whether a peer which stays configured is established or completing its handshake.
func (instance *Instance) hasSession(ctx context.Context, resolvedPeers []Peer) bool {
	addresses := xslices.ToSet(xslices.Map(resolvedPeers, func(peer Peer) string { return peer.Address }))

	var session bool

	if err := instance.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(peer *gobgpapi.Peer) {
		if _, configured := addresses[peer.GetConf().GetNeighborAddress()]; !configured {
			return
		}

		switch peer.GetState().GetSessionState() { //nolint:exhaustive // only sessions with an OPEN exchange in progress or done
		case gobgpapi.PeerState_SESSION_STATE_OPENSENT, gobgpapi.PeerState_SESSION_STATE_OPENCONFIRM, gobgpapi.PeerState_SESSION_STATE_ESTABLISHED:
			session = true
		}
	}); err != nil {
		// only a canceled context fails it: keep the router ID rather than rebuild the server
		return true
	}

	return session
}

// listenAddresses returns the wildcard addresses to listen on.
var listenAddresses = sync.OnceValue(func() []string {
	addrs := []string{
		netip.IPv4Unspecified().String(),
	}

	if fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0); err == nil {
		unix.Close(fd) //nolint:errcheck

		addrs = append(addrs, netip.IPv6Unspecified().String())
	}

	return addrs
})

// ReconcileOriginated diffs the desired advertised prefixes against what is currently originated.
func (instance *Instance) ReconcileOriginated(advertised []netip.Prefix) error {
	desired := make(map[netip.Prefix]struct{}, len(advertised))

	for _, prefix := range advertised {
		desired[prefix] = struct{}{}

		if _, ok := instance.originated[prefix]; ok {
			continue
		}

		path, err := BuildOriginatedPath(prefix)
		if err != nil {
			return err
		}

		if _, err = instance.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}}); err != nil {
			return fmt.Errorf("error adding path %s: %w", prefix, err)
		}

		instance.originated[prefix] = struct{}{}

		// GoBGP keys local paths by prefix, so an imported path for it was just replaced along with its UUID
		delete(instance.imported, prefix)
	}

	for prefix := range instance.originated {
		if _, ok := desired[prefix]; ok {
			continue
		}

		path, err := BuildOriginatedPath(prefix)
		if err != nil {
			return err
		}

		if err = instance.server.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}}); err != nil {
			return fmt.Errorf("error deleting path %s: %w", prefix, err)
		}

		delete(instance.originated, prefix)
	}

	return nil
}

// SetOutputState records the successfully reconciled configuration used to build controller outputs.
func (instance *Instance) SetOutputState(
	connected []netip.Prefix,
	imports []network.BGPImportRouteSpec,
	table nethelpers.RoutingTable,
	source netip.Addr,
	localASN uint32,
	installRoutes bool,
) {
	instance.connected = slices.Clone(connected)
	instance.imports = cloneImportRoutes(imports)
	instance.table = table
	instance.source = source
	instance.localASN = localASN
	instance.installRoutes = installRoutes
}

// reconcilePeers adds, removes and updates peers to match the resolved set.
// A peer GoBGP rejects is skipped and retried on the next reconcile; the other peers are still reconciled.
//
// Changed peers are updated in place: GoBGP applies BFD and timer changes to the live session, and resets it
// only for changes which need a new OPEN message (e.g. ASN or transport settings).
func (instance *Instance) reconcilePeers(
	ctx context.Context,
	config network.BGPInstanceConfigSpec,
	resolved []Peer,
) error {
	var errs error

	desired := make(map[string]string, len(resolved))

	for _, peer := range resolved {
		desired[peer.Address] = PeerKey(peer)
	}

	for address := range instance.peers {
		if _, ok := desired[address]; ok {
			continue
		}

		if err := instance.server.DeletePeer(ctx, &gobgpapi.DeletePeerRequest{Address: address}); err != nil {
			errs = errors.Join(errs, fmt.Errorf("error deleting BGP peer %s: %w", address, err))

			continue
		}

		delete(instance.peers, address)
	}

	for _, peer := range resolved {
		hash, ok := instance.peers[peer.Address]

		switch {
		case !ok:
			if err := instance.server.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: BuildPeer(peer, config.Multipath)}); err != nil {
				errs = errors.Join(errs, fmt.Errorf("error adding BGP peer %s: %w", peer.Address, err))

				continue
			}
		case hash != desired[peer.Address]:
			if _, err := instance.server.UpdatePeer(ctx, &gobgpapi.UpdatePeerRequest{Peer: BuildPeer(peer, config.Multipath)}); err != nil {
				errs = errors.Join(errs, fmt.Errorf("error updating BGP peer %s: %w", peer.Address, err))

				continue
			}
		}

		instance.peers[peer.Address] = desired[peer.Address]
	}

	return errs
}
