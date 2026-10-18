// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"context"
	"fmt"
	"log/slog" //nolint:loglinter // GoBGP's logger compatibility interface requires slog.
	"maps"
	"net/netip"
	"slices"
	"sync"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
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
	watchCancel   context.CancelFunc
	originated    map[netip.Prefix]struct{}
	imported      map[netip.Prefix]importedPath
	advertised    []netip.Prefix
	imports       []network.BGPImportRouteSpec
	table         nethelpers.RoutingTable
	source        netip.Addr
	localASN      uint32
	installRoutes bool
	peers         map[string]string
	peerIfaces    map[netip.Addr]string

	// replacing is the established peer replaced last because its configuration changed: the other changed
	// established peers keep their old configuration (and keep carrying traffic) until it is back or
	// replacingDeadline passes.
	replacing          string
	replacingDeadline  time.Time
	replacingTimer     *time.Timer
	peerReplaceTimeout time.Duration

	// peerStates overrides listPeerStates in tests.
	peerStates func(ctx context.Context) (established, synced map[string]bool)
}

// defaultPeerReplaceTimeout is how long a changed peer waits for the previously replaced peer to come back.
const defaultPeerReplaceTimeout = 30 * time.Second

// NewInstance creates an initialized, stopped BGP instance.
func NewInstance() *Instance {
	return &Instance{
		originated: map[netip.Prefix]struct{}{},
		imported:   map[netip.Prefix]importedPath{},
		peers:      map[string]string{},
		peerIfaces: map[netip.Addr]string{},

		peerReplaceTimeout: defaultPeerReplaceTimeout,
	}
}

// Running reports whether the instance has an active GoBGP server.
func (instance *Instance) Running() bool {
	return instance.server != nil
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

	if instance.replacingTimer != nil {
		instance.replacingTimer.Stop()
		instance.replacingTimer = nil
	}

	instance.clearState()
}

func (instance *Instance) clearState() {
	instance.originated = map[netip.Prefix]struct{}{}
	instance.imported = map[netip.Prefix]importedPath{}
	instance.advertised = nil
	instance.imports = nil
	instance.table = 0
	instance.source = netip.Addr{}
	instance.localASN = 0
	instance.installRoutes = false
	instance.peers = map[string]string{}
	instance.peerIfaces = map[netip.Addr]string{}
	instance.replacing = ""
	instance.replacingDeadline = time.Time{}
}

// EnsureServer (re)creates the GoBGP server when server-level configuration changes, then
// reconciles its peer set incrementally.
func (instance *Instance) EnsureServer(
	ctx context.Context,
	logger *zap.Logger,
	config network.BGPInstanceConfigSpec,
	routerID netip.Addr,
	vrfIndex uint32,
	resolvedPeers []Peer,
	peerIfaces map[netip.Addr]string,
	listenPort int32,
	signal func(),
) error {
	key := ServerKey(config.LocalASN, routerID, config.Multipath, config.MaxPaths, config.VRF, vrfIndex, config.VRFTable, listenPort)

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
			// a replaced peer is back once it sent End-of-RIB, see reconcilePeers
			OnPathEor: func(*apiutil.Path, time.Time) {
				signal()
			},
		}, gobgpsrv.WatchBestPath(true), gobgpsrv.WatchPeer(), gobgpsrv.WatchEor(false)); err != nil {
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

	instance.peerIfaces = peerIfaces

	return instance.reconcilePeers(ctx, logger, config, resolvedPeers, signal)
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
	advertised []netip.Prefix,
	imports []network.BGPImportRouteSpec,
	table nethelpers.RoutingTable,
	source netip.Addr,
	localASN uint32,
	installRoutes bool,
) {
	instance.advertised = slices.Clone(advertised)
	instance.imports = cloneImportRoutes(imports)
	instance.table = table
	instance.source = source
	instance.localASN = localASN
	instance.installRoutes = installRoutes
}

// reconcilePeers adds, removes and replaces (delete + add) peers to match the resolved set.
//
// Changed peers that are established are replaced one at a time: the next one is replaced only once the
// previous one is established again and has sent End-of-RIB, or after defaultPeerReplaceTimeout. Otherwise a change
// that applies to every neighbor (e.g. BFD timers) would take all sessions down at once.
func (instance *Instance) reconcilePeers(
	ctx context.Context,
	logger *zap.Logger,
	config network.BGPInstanceConfigSpec,
	resolved []Peer,
	signal func(),
) error {
	desired := make(map[string]string, len(resolved))

	for _, peer := range resolved {
		desired[peer.Address] = PeerKey(peer)
	}

	if err := instance.removeChangedPeers(ctx, logger, desired, signal); err != nil {
		return err
	}

	for _, peer := range resolved {
		if _, ok := instance.peers[peer.Address]; ok {
			continue
		}

		if err := instance.server.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: BuildPeer(peer, config.Multipath)}); err != nil {
			return fmt.Errorf("error adding BGP peer: %w", err)
		}

		instance.peers[peer.Address] = desired[peer.Address]
	}

	return nil
}

// removeChangedPeers deletes the peers that are gone or whose configuration changed, holding back all but one of
// the changed established peers (see reconcilePeers).
func (instance *Instance) removeChangedPeers(ctx context.Context, logger *zap.Logger, desired map[string]string, signal func()) error {
	established, synced := instance.listPeerStates(ctx)

	instance.finishReplacing(desired, synced)

	for _, address := range slices.Sorted(maps.Keys(instance.peers)) {
		hash, ok := desired[address]
		if hash == instance.peers[address] {
			continue
		}

		if ok && established[address] && !instance.startReplacing(logger, address, signal) {
			continue
		}

		if err := instance.server.DeletePeer(ctx, &gobgpapi.DeletePeerRequest{Address: address}); err != nil {
			return fmt.Errorf("error deleting BGP peer: %w", err)
		}

		delete(instance.peers, address)
	}

	return nil
}

// finishReplacing forgets the peer being replaced once it is removed, back with all its routes, or out of time.
func (instance *Instance) finishReplacing(desired map[string]string, synced map[string]bool) {
	if _, ok := desired[instance.replacing]; instance.replacing != "" &&
		(!ok || synced[instance.replacing] || !time.Now().Before(instance.replacingDeadline)) {
		instance.replacing = ""
	}
}

// startReplacing records address as the peer being replaced, unless another one is still being replaced.
func (instance *Instance) startReplacing(logger *zap.Logger, address string, signal func()) bool {
	if instance.replacing != "" {
		logger.Info("BGP peer configuration changed, waiting for the previously replaced peer to come back",
			zap.String("peer", address), zap.String("replaced_peer", instance.replacing), zap.Time("until", instance.replacingDeadline))

		return false
	}

	logger.Info("replacing BGP peer with the changed configuration", zap.String("peer", address))

	instance.replacing = address
	instance.replacingDeadline = time.Now().Add(instance.peerReplaceTimeout)

	if instance.replacingTimer != nil {
		instance.replacingTimer.Stop()
	}

	instance.replacingTimer = time.AfterFunc(instance.peerReplaceTimeout, signal)

	return true
}

// listPeerStates reports the established peers, and which of them sent End-of-RIB for all their families.
func (instance *Instance) listPeerStates(ctx context.Context) (established, synced map[string]bool) {
	if instance.peerStates != nil {
		return instance.peerStates(ctx)
	}

	established, synced = map[string]bool{}, map[string]bool{}

	if err := instance.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(peer *gobgpapi.Peer) {
		if peer.GetState().GetSessionState() != gobgpapi.PeerState_SESSION_STATE_ESTABLISHED {
			return
		}

		address := peer.GetConf().GetNeighborAddress()
		established[address] = true
		synced[address] = !slices.ContainsFunc(peer.GetAfiSafis(), func(afiSafi *gobgpapi.AfiSafi) bool {
			return afiSafi.GetState().GetEnabled() && !afiSafi.GetMpGracefulRestart().GetState().GetEndOfRibReceived()
		})
	}); err != nil {
		// without peer states, every changed peer is replaced right away
		return map[string]bool{}, map[string]bool{}
	}

	return established, synced
}
