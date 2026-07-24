// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"iter"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
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

	instances map[resource.ID]*internalbgp.Instance

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
	ctrl.instances = map[resource.ID]*internalbgp.Instance{}

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
		instance.Stop()
	}

	ctrl.instances = map[resource.ID]*internalbgp.Instance{}
}

type bgpInstanceOutputs struct {
	name         resource.ID
	table        nethelpers.RoutingTable
	source       netip.Addr
	learned      map[netip.Prefix][]network.RouteNextHop
	peerStatuses []network.BGPPeerStatusSpec
}

func instanceOutputs(ctx context.Context, name resource.ID, instance *internalbgp.Instance) bgpInstanceOutputs {
	snapshot := instance.Snapshot(ctx)

	peerStatuses := snapshot.PeerStatuses
	for i := range peerStatuses {
		peerStatuses[i].Instance = name
	}

	return bgpInstanceOutputs{
		name:         name,
		table:        snapshot.Table,
		source:       snapshot.Source,
		learned:      snapshot.Learned,
		peerStatuses: peerStatuses,
	}
}

func bgpLinkStatusSpecs(statuses iter.Seq[*network.LinkStatus]) map[string]network.LinkStatusSpec {
	result := map[string]network.LinkStatusSpec{}

	for status := range statuses {
		result[status.Metadata().ID()] = *status.TypedSpec()
	}

	return result
}

func bgpAddressStatusSpecs(statuses iter.Seq[*network.AddressStatus]) []network.AddressStatusSpec {
	var result []network.AddressStatusSpec

	for status := range statuses {
		result = append(result, *status.TypedSpec())
	}

	return result
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

	runtimeState := internalbgp.NewRuntimeState(
		bgpLinkStatusSpecs(linkStatuses.All()),
		bgpAddressStatusSpecs(addressStatuses.All()),
	)
	neighborResolver := internalbgp.NewNeighborResolver()

	desired := map[resource.ID]struct{}{}

	for configResource := range configs.All() {
		name := configResource.Metadata().ID()
		desired[name] = struct{}{}

		instance, ok := ctrl.instances[name]
		if !ok {
			instance = internalbgp.NewInstance()
			ctrl.instances[name] = instance
		}

		resolved, resolveErr := runtimeState.Resolve(*configResource.TypedSpec())
		if resolveErr != nil {
			instanceLogger := logger.With(zap.String("instance", name))
			instanceLogger.Warn("BGP runtime configuration is not ready, preserving the running instance", zap.Error(resolveErr))

			continue
		}

		instanceLogger := logger.With(zap.String("instance", name))

		if !resolved.RouterID.IsValid() {
			instanceLogger.Warn("BGP router-id could not be determined, preserving the running instance")

			continue
		}

		peers, peerIfaces := neighborResolver.ResolvePeers(resolved.Spec, runtimeState, instanceLogger)

		if err = instance.EnsureServer(
			ctx,
			instanceLogger,
			resolved.Spec,
			resolved.RouterID,
			peers,
			peerIfaces,
			ctrl.effectiveListenPort(),
			ctrl.signal,
		); err != nil {
			return fmt.Errorf("error configuring BGP instance %q: %w", name, err)
		}

		if err = instance.ReconcileOriginated(resolved.AdvertisedPrefixes); err != nil {
			return fmt.Errorf("error originating routes for BGP instance %q: %w", name, err)
		}

		instance.SetOutputState(
			resolved.AdvertisedPrefixes,
			resolved.Spec.ImportRoutes,
			resolved.Spec.VRFTable,
			resolved.Spec.RouteSource,
			resolved.Spec.LocalASN,
			resolved.Spec.InstallRoutes,
		)
	}

	for name, instance := range ctrl.instances {
		if _, exists := desired[name]; exists {
			continue
		}

		instance.Stop()
		delete(ctrl.instances, name)
	}

	instanceNames := make([]resource.ID, 0, len(desired))
	for name := range desired {
		instanceNames = append(instanceNames, name)
	}

	slices.Sort(instanceNames)

	for _, name := range instanceNames {
		instance := ctrl.instances[name]
		if instance == nil || !instance.Running() {
			continue
		}

		desiredCount, ready, importErr := internalbgp.ReconcileImportedPaths(name, instance, ctrl.instances, desired)
		if importErr != nil {
			return fmt.Errorf("error resolving route imports for BGP instance %q: %w", name, importErr)
		}

		if !ready {
			logger.With(zap.String("instance", name)).Warn("BGP route import source is not ready, preserving imported paths")

			continue
		}

		logger.Debug(
			"reconciled BGP route imports",
			zap.String("instance", name),
			zap.Int("desired", desiredCount),
			zap.Int("current", instance.ImportedCount()),
		)
	}

	outputs := make([]bgpInstanceOutputs, 0, len(instanceNames))

	for _, name := range instanceNames {
		instance := ctrl.instances[name]
		if instance == nil || !instance.Running() {
			continue
		}

		output := instanceOutputs(ctx, name, instance)
		outputs = append(outputs, output)

		logger.Debug(
			"prepared BGP instance outputs",
			zap.String("instance", name),
			zap.Bool("install_routes", instance.InstallRoutes()),
			zap.Int("learned_routes", len(output.learned)),
			zap.Int("imported_routes", instance.ImportedCount()),
			zap.Int("peer_statuses", len(output.peerStatuses)),
		)
	}

	return ctrl.writeOutputs(ctx, r, outputs)
}

func (ctrl *BGPController) effectiveListenPort() int32 {
	if ctrl.ListenPort != 0 {
		return ctrl.ListenPort
	}

	return constants.BGPDefaultPort
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
