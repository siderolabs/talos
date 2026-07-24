// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"context"
	"fmt"
	"log/slog" //nolint:loglinter // GoBGP's logger compatibility interface requires slog.
	"net/netip"
	"slices"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"

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
}

// NewInstance creates an initialized, stopped BGP instance.
func NewInstance() *Instance {
	return &Instance{
		originated: map[netip.Prefix]struct{}{},
		imported:   map[netip.Prefix]importedPath{},
		peers:      map[string]string{},
		peerIfaces: map[netip.Addr]string{},
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
}

// EnsureServer (re)creates the GoBGP server when server-level configuration changes, then
// reconciles its peer set incrementally.
func (instance *Instance) EnsureServer(
	ctx context.Context,
	logger *zap.Logger,
	config network.BGPInstanceConfigSpec,
	routerID netip.Addr,
	resolvedPeers []Peer,
	peerIfaces map[netip.Addr]string,
	listenPort int32,
	signal func(),
) error {
	key := ServerKey(config.LocalASN, routerID, config.Multipath, config.MaxPaths, config.VRF, config.VRFTable, listenPort)

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

	instance.peerIfaces = peerIfaces

	return instance.reconcilePeers(ctx, config, resolvedPeers)
}

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

func (instance *Instance) reconcilePeers(
	ctx context.Context,
	config network.BGPInstanceConfigSpec,
	resolved []Peer,
) error {
	desired := make(map[string]string, len(resolved))

	for _, peer := range resolved {
		desired[peer.Address] = PeerKey(peer)
	}

	for address, hash := range instance.peers {
		if desired[address] == hash {
			continue
		}

		if err := instance.server.DeletePeer(ctx, &gobgpapi.DeletePeerRequest{Address: address}); err != nil {
			return fmt.Errorf("error deleting BGP peer: %w", err)
		}

		delete(instance.peers, address)
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
