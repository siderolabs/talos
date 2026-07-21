// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"net/netip"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jsimonetti/rtnetlink/v2"
	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// BGPController runs embedded GoBGP routing instances driven by projected BGPInstanceConfig resources.
//
// It originates the addresses of the configured links as host routes, installs the routes it
// learns from its neighbors as network.RouteSpec resources, and exposes peer state as
// network.BGPPeerStatus resources.
type BGPController struct {
	// ListenPort overrides the default BGP port when non-zero. Negative values disable listeners.
	// It is used by focused controller tests to avoid binding a host port.
	ListenPort int32

	instances map[resource.ID]*bgpInstance

	reconcileCh chan struct{}
}

type bgpInstance struct {
	server      *gobgpsrv.BgpServer
	serverKey   string
	watchCancel context.CancelFunc
	originated  map[netip.Prefix]struct{}
	advertised  []netip.Prefix
	table       nethelpers.RoutingTable
	source      netip.Addr
	localASN    uint32
	// peers maps an added peer's gobgp address to a hash of its configuration, so peers can be
	// reconciled incrementally (added/removed/updated) without restarting the whole server.
	peers map[string]string
	// peerIfaces maps a resolved unnumbered peer's link-local address to its link, used to set
	// the egress link on routes learned with a link-local next-hop.
	peerIfaces map[netip.Addr]string
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
			Type:      network.BGPInstanceConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.AddressStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
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
	ctrl.instances = map[resource.ID]*bgpInstance{}

	defer ctrl.stopServers()

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

func (ctrl *BGPController) stopServers() {
	for _, instance := range ctrl.instances {
		instance.stopServer()
	}

	ctrl.instances = map[resource.ID]*bgpInstance{}
}

func (instance *bgpInstance) stopServer() {
	if instance.watchCancel != nil {
		instance.watchCancel()
		instance.watchCancel = nil
	}

	if instance.server != nil {
		instance.server.Stop()
		instance.server = nil
	}

	instance.serverKey = ""
	instance.originated = map[netip.Prefix]struct{}{}
	instance.advertised = nil
	instance.table = 0
	instance.source = netip.Addr{}
	instance.localASN = 0
	instance.peers = map[string]string{}
	instance.peerIfaces = map[netip.Addr]string{}
}

type bgpInstanceOutputs struct {
	name         resource.ID
	table        nethelpers.RoutingTable
	source       netip.Addr
	learned      map[netip.Prefix][]network.RouteNextHop
	peerStatuses []network.BGPPeerStatusSpec
}

type bgpRuntimeState struct {
	resolver        *network.LinkResolver
	statusByName    map[string]*network.LinkStatus
	statusByIndex   map[uint32]*network.LinkStatus
	addressLinks    map[netip.Addr][]string
	addressesByLink map[uint32]map[netip.Addr]struct{}
	addressStatuses []*network.AddressStatus
}

func newBGPRuntimeState(
	linkStatuses func() iter.Seq[*network.LinkStatus],
	addressStatuses func() iter.Seq[*network.AddressStatus],
) *bgpRuntimeState {
	state := &bgpRuntimeState{
		resolver:        network.NewLinkResolver(linkStatuses),
		statusByName:    map[string]*network.LinkStatus{},
		statusByIndex:   map[uint32]*network.LinkStatus{},
		addressLinks:    map[netip.Addr][]string{},
		addressesByLink: map[uint32]map[netip.Addr]struct{}{},
	}

	for status := range linkStatuses() {
		state.statusByName[status.Metadata().ID()] = status
		state.statusByIndex[status.TypedSpec().Index] = status
	}

	for status := range addressStatuses() {
		state.addressStatuses = append(state.addressStatuses, status)

		spec := status.TypedSpec()
		address := spec.Address.Addr()
		linkName := state.resolver.Resolve(spec.LinkName)
		state.addressLinks[address] = append(state.addressLinks[address], linkName)

		if state.addressesByLink[spec.LinkIndex] == nil {
			state.addressesByLink[spec.LinkIndex] = map[netip.Addr]struct{}{}
		}

		state.addressesByLink[spec.LinkIndex][address] = struct{}{}
	}

	return state
}

func (state *bgpRuntimeState) resolve(spec *network.BGPInstanceConfigSpec) (network.BGPInstanceConfigSpec, error) {
	resolved := *spec
	resolved.VRF = state.resolver.Resolve(spec.VRF)
	resolved.AdvertiseLinks = slices.Clone(spec.AdvertiseLinks)
	resolved.Neighbors = slices.Clone(spec.Neighbors)

	if resolved.VRF != "" {
		status, exists := state.statusByName[resolved.VRF]
		if !exists {
			return network.BGPInstanceConfigSpec{}, fmt.Errorf("VRF link %q is not ready", resolved.VRF)
		}

		if status.TypedSpec().Kind != network.LinkKindVRF {
			return network.BGPInstanceConfigSpec{}, fmt.Errorf("link %q is not a VRF", resolved.VRF)
		}
	}

	if resolved.RouteSource.IsValid() {
		if err := state.validateRouteSource(resolved.RouteSource, resolved.VRF); err != nil {
			return network.BGPInstanceConfigSpec{}, fmt.Errorf("route source: %w", err)
		}
	}

	if err := state.resolveLinks(&resolved); err != nil {
		return network.BGPInstanceConfigSpec{}, err
	}

	return resolved, nil
}

func (state *bgpRuntimeState) resolveLinks(resolved *network.BGPInstanceConfigSpec) error {
	for i, link := range resolved.AdvertiseLinks {
		resolved.AdvertiseLinks[i] = state.resolver.Resolve(link)

		if err := state.validateLinkDomain(resolved.AdvertiseLinks[i], resolved.VRF); err != nil {
			return fmt.Errorf("advertised link: %w", err)
		}
	}

	for i := range resolved.Neighbors {
		if resolved.Neighbors[i].Link == "" {
			continue
		}

		resolved.Neighbors[i].Link = state.resolver.Resolve(resolved.Neighbors[i].Link)

		if err := state.validateLinkDomain(resolved.Neighbors[i].Link, resolved.VRF); err != nil {
			return fmt.Errorf("neighbor link: %w", err)
		}
	}

	return nil
}

func (state *bgpRuntimeState) validateRouteSource(source netip.Addr, vrf string) error {
	links := state.addressLinks[source]
	if len(links) == 0 {
		return fmt.Errorf("address %s is not ready", source)
	}

	var domainErr error

	for _, link := range links {
		if err := state.validateLinkDomain(link, vrf); err != nil {
			domainErr = err

			continue
		}

		return nil
	}

	return domainErr
}

func (state *bgpRuntimeState) validateLinkDomain(linkName, wantVRF string) error {
	status, exists := state.statusByName[linkName]
	if !exists {
		return fmt.Errorf("link %q is not ready", linkName)
	}

	actualVRF, err := linkVRF(status, state.statusByIndex)
	if err != nil {
		return err
	}

	if actualVRF != wantVRF {
		if wantVRF == "" {
			return fmt.Errorf("link %q belongs to VRF %q, not the default routing domain", linkName, actualVRF)
		}

		return fmt.Errorf("link %q belongs to VRF %q, not VRF %q", linkName, actualVRF, wantVRF)
	}

	return nil
}

func linkVRF(status *network.LinkStatus, statusByIndex map[uint32]*network.LinkStatus) (string, error) {
	seen := map[uint32]struct{}{}

	for status != nil {
		if status.TypedSpec().Kind == network.LinkKindVRF {
			return status.Metadata().ID(), nil
		}

		masterIndex := status.TypedSpec().MasterIndex
		if masterIndex == 0 {
			return "", nil
		}

		if _, exists := seen[masterIndex]; exists {
			return "", fmt.Errorf("link %q has a cyclic master chain", status.Metadata().ID())
		}

		seen[masterIndex] = struct{}{}

		status = statusByIndex[masterIndex]
		if status == nil {
			return "", fmt.Errorf("link master index %d is not ready", masterIndex)
		}
	}

	return "", nil
}

func (instance *bgpInstance) outputs(ctx context.Context, name resource.ID) bgpInstanceOutputs {
	peerStatuses := instance.listPeers(ctx, instance.localASN)
	for i := range peerStatuses {
		peerStatuses[i].Instance = name
	}

	return bgpInstanceOutputs{
		name:         name,
		table:        instance.table,
		source:       instance.source,
		learned:      instance.listLearnedRoutes(instance.advertised, instance.peerIfaces),
		peerStatuses: peerStatuses,
	}
}

//nolint:gocyclo,cyclop
func (ctrl *BGPController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	configs, err := safe.ReaderListAll[*network.BGPInstanceConfig](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing BGP instance configs: %w", err)
	}

	linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing link statuses: %w", err)
	}

	addressStatuses, err := safe.ReaderListAll[*network.AddressStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing address statuses: %w", err)
	}

	runtimeState := newBGPRuntimeState(linkStatuses.All, addressStatuses.All)

	desired := map[resource.ID]struct{}{}

	var outputs []bgpInstanceOutputs

	for configResource := range configs.All() {
		name := configResource.Metadata().ID()
		desired[name] = struct{}{}

		instance := ctrl.instances[name]
		if instance == nil {
			instance = &bgpInstance{}
			instance.stopServer()
			ctrl.instances[name] = instance
		}

		bgpConfig, resolveErr := runtimeState.resolve(configResource.TypedSpec())
		if resolveErr != nil {
			instanceLogger := logger.With(zap.String("instance", name))
			instanceLogger.Warn("BGP runtime configuration is not ready, preserving the running instance", zap.Error(resolveErr))

			if instance.server != nil {
				outputs = append(outputs, instance.outputs(ctx, name))
			}

			continue
		}

		advertised := ctrl.advertisedPrefixes(&bgpConfig, runtimeState)
		routerID := ctrl.routerID(&bgpConfig, advertised)
		instanceLogger := logger.With(zap.String("instance", name))

		if !routerID.IsValid() {
			instanceLogger.Warn("BGP router-id could not be determined, preserving the running instance")

			if instance.server != nil {
				outputs = append(outputs, instance.outputs(ctx, name))
			}

			continue
		}

		if err = instance.ensureServer(ctx, instanceLogger, &bgpConfig, runtimeState, routerID, ctrl.effectiveListenPort(), ctrl.signal); err != nil {
			return fmt.Errorf("error configuring BGP instance %q: %w", name, err)
		}

		if err = instance.reconcileOriginated(advertised); err != nil {
			return fmt.Errorf("error originating routes for BGP instance %q: %w", name, err)
		}

		instance.advertised = slices.Clone(advertised)
		instance.table = bgpConfig.VRFTable
		instance.source = bgpConfig.RouteSource
		instance.localASN = bgpConfig.LocalASN

		outputs = append(outputs, instance.outputs(ctx, name))
	}

	for name, instance := range ctrl.instances {
		if _, exists := desired[name]; exists {
			continue
		}

		instance.stopServer()
		delete(ctrl.instances, name)
	}

	return ctrl.writeOutputs(ctx, r, outputs)
}

func (ctrl *BGPController) effectiveListenPort() int32 {
	if ctrl.ListenPort != 0 {
		return ctrl.ListenPort
	}

	return constants.BGPDefaultPort
}

// ensureServer (re)creates this instance's gobgp server when server-level configuration changes, then
// reconciles its peer set incrementally.
func (instance *bgpInstance) ensureServer(
	ctx context.Context,
	logger *zap.Logger,
	bgpConfig *network.BGPInstanceConfigSpec,
	runtimeState *bgpRuntimeState,
	routerID netip.Addr,
	listenPort int32,
	signal func(),
) error {
	// resolve neighbors (unnumbered peers resolve their link-local from the kernel neighbor table,
	// populated via Router Advertisements); skip peers not yet discovered (reconciled on the next event).
	instance.peerIfaces = map[netip.Addr]string{}

	var resolved []internalbgp.Peer

	for _, neighbor := range bgpConfig.Neighbors {
		peer, ok := resolveNeighborPeer(neighbor, runtimeState, logger)
		if !ok {
			logger.Debug("unnumbered BGP peer not yet discovered, will retry", zap.String("link", neighbor.Link))

			continue
		}

		resolved = append(resolved, peer)

		if peer.LinkLocal.IsValid() {
			instance.peerIfaces[peer.LinkLocal] = peer.Link
		}

		if bgpConfig.VRF != "" {
			resolved[len(resolved)-1].BindInterface = bgpConfig.VRF
		} else if peer.Link != "" {
			resolved[len(resolved)-1].BindInterface = peer.Link
		}
	}

	key := internalbgp.ServerKey(bgpConfig.LocalASN, routerID, bgpConfig.Multipath, bgpConfig.MaxPaths, bgpConfig.VRF, bgpConfig.VRFTable, listenPort)

	if instance.server == nil || instance.serverKey != key {
		instance.stopServer()

		// route gobgp's logs into the controller's zap logger (gobgp's LoggerOption requires an *slog.Logger);
		// the level var gates gobgp at warn+ to keep it quiet, zap applies the final filtering.
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelWarn)

		srv := gobgpsrv.NewBgpServer(gobgpsrv.LoggerOption(slog.New(zapslog.NewHandler(logger.Core())), lvl))

		go srv.Serve()

		global := &gobgpapi.Global{
			Asn:              bgpConfig.LocalASN,
			RouterId:         routerID.String(),
			ListenPort:       listenPort,
			UseMultiplePaths: bgpConfig.Multipath,
			BindToDevice:     bgpConfig.VRF,
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

		logger.Info("started embedded BGP speaker", zap.Uint32("asn", bgpConfig.LocalASN), zap.Stringer("router_id", routerID))
	}

	return instance.reconcilePeers(ctx, bgpConfig, resolved)
}

// reconcilePeers diffs the resolved neighbor set against the peers currently configured on the running
// gobgp server, adding new (or changed) peers and removing stale ones — without restarting the server.
func (instance *bgpInstance) reconcilePeers(ctx context.Context, bgpConfig *network.BGPInstanceConfigSpec, resolved []internalbgp.Peer) error {
	desired := make(map[string]string, len(resolved))

	for _, peer := range resolved {
		desired[peer.Address] = internalbgp.PeerKey(peer)
	}

	// remove peers that are gone or whose configuration changed (re-added below).
	for address, hash := range instance.peers {
		if desired[address] == hash {
			continue
		}

		if err := instance.server.DeletePeer(ctx, &gobgpapi.DeletePeerRequest{Address: address}); err != nil {
			return fmt.Errorf("error deleting BGP peer: %w", err)
		}

		delete(instance.peers, address)
	}

	// add new (or changed) peers.
	for _, peer := range resolved {
		if _, ok := instance.peers[peer.Address]; ok {
			continue
		}

		if err := instance.server.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: internalbgp.BuildPeer(peer, bgpConfig.Multipath)}); err != nil {
			return fmt.Errorf("error adding BGP peer: %w", err)
		}

		instance.peers[peer.Address] = desired[peer.Address]
	}

	return nil
}

// reconcileOriginated diffs the desired advertised prefixes against what is currently originated.
func (instance *bgpInstance) reconcileOriginated(advertised []netip.Prefix) error {
	desired := make(map[netip.Prefix]struct{}, len(advertised))

	for _, prefix := range advertised {
		desired[prefix] = struct{}{}

		if _, ok := instance.originated[prefix]; ok {
			continue
		}

		path, err := internalbgp.BuildOriginatedPath(prefix)
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

		path, err := internalbgp.BuildOriginatedPath(prefix)
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

// listLearnedRoutes builds the set of best-path routes learned from peers, keyed by destination.
//
// Locally originated prefixes are excluded.
//
//nolint:gocyclo
func (instance *bgpInstance) listLearnedRoutes(advertised []netip.Prefix, peerIfaces map[netip.Addr]string) map[netip.Prefix][]network.RouteNextHop {
	learned := map[netip.Prefix][]network.RouteNextHop{}

	advertisedSet := make(map[netip.Prefix]struct{}, len(advertised))
	for _, prefix := range advertised {
		advertisedSet[prefix] = struct{}{}
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
func (instance *bgpInstance) listPeers(ctx context.Context, localASN uint32) []network.BGPPeerStatusSpec {
	var peers []network.BGPPeerStatusSpec

	if err := instance.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(p *gobgpapi.Peer) {
		peers = append(peers, internalbgp.PeerStatus(p, localASN))
	}); err != nil {
		return nil
	}

	return peers
}

// writeOutputs reconciles RouteSpec and BGPPeerStatus resources owned by this controller.
func (ctrl *BGPController) writeOutputs(ctx context.Context, r controller.Runtime, instances []bgpInstanceOutputs) error {
	r.StartTrackingOutputs()

	for _, instance := range instances {
		for prefix, nexthops := range instance.learned {
			spec := internalbgp.RouteSpec(prefix, nexthops, instance.source, instance.table)

			id := "bgp/" + instance.name + "/" + network.RouteID(spec.Table, spec.Family, spec.Destination, spec.Gateway, spec.Priority, spec.OutLinkName)

			if err := safe.WriterModify(ctx, r, network.NewRouteSpec(network.ConfigNamespaceName, id), func(route *network.RouteSpec) error {
				*route.TypedSpec() = spec

				return nil
			}); err != nil {
				return fmt.Errorf("error writing route spec for BGP instance %q: %w", instance.name, err)
			}
		}

		for _, peer := range instance.peerStatuses {
			id := instance.name + "/" + peer.Peer

			if err := safe.WriterModify(ctx, r, network.NewBGPPeerStatus(network.NamespaceName, id), func(status *network.BGPPeerStatus) error {
				*status.TypedSpec() = peer

				return nil
			}); err != nil {
				return fmt.Errorf("error writing BGP peer status for instance %q: %w", instance.name, err)
			}
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
func (ctrl *BGPController) advertisedPrefixes(bgpConfig *network.BGPInstanceConfigSpec, runtimeState *bgpRuntimeState) []netip.Prefix {
	links := make(map[string]struct{}, len(bgpConfig.AdvertiseLinks))
	for _, link := range bgpConfig.AdvertiseLinks {
		links[link] = struct{}{}
	}

	if len(links) == 0 {
		return nil
	}

	var prefixes []netip.Prefix

	for _, address := range runtimeState.addressStatuses {
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

	slices.SortFunc(prefixes, func(left, right netip.Prefix) int {
		return left.Addr().Compare(right.Addr())
	})

	return prefixes
}

// routerID picks the BGP router-id: configured value, or the first advertised IPv4 address.
func (ctrl *BGPController) routerID(bgpConfig *network.BGPInstanceConfigSpec, advertised []netip.Prefix) netip.Addr {
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
func resolveNeighborPeer(
	neighbor network.BGPNeighborConfigSpec,
	runtimeState *bgpRuntimeState,
	logger *zap.Logger,
) (internalbgp.Peer, bool) {
	peer := internalbgp.Peer{Config: neighbor}

	if addr := neighbor.Address; addr.IsValid() {
		peer.Address = addr.String()

		return peer, true
	}

	iface := neighbor.Link
	if iface == "" {
		return internalbgp.Peer{}, false
	}

	linkStatus, ok := runtimeState.statusByName[iface]
	if !ok {
		return internalbgp.Peer{}, false
	}

	lla, ok := linkLocalNeighbor(
		iface,
		linkStatus.TypedSpec().Index,
		runtimeState.addressesByLink[linkStatus.TypedSpec().Index],
		logger,
	)
	if !ok {
		return internalbgp.Peer{}, false
	}

	peer.Address = lla.String() + "%" + iface
	peer.LinkLocal = lla
	peer.Link = iface

	return peer, true
}

// linkLocalNeighbor returns the single IPv6 link-local neighbor on the interface (the unnumbered peer).
// It returns false unless exactly one such neighbor is present (point-to-point assumption).
//
//nolint:gocyclo,cyclop
func linkLocalNeighbor(
	iface string,
	index uint32,
	ownAddrs map[netip.Addr]struct{},
	logger *zap.Logger,
) (netip.Addr, bool) {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return netip.Addr{}, false
	}

	defer conn.Close() //nolint:errcheck

	neighbors, err := conn.Neigh.List()
	if err != nil {
		return netip.Addr{}, false
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
