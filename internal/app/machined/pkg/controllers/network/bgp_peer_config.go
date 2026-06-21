// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// BGPPeerConfigController renders the machine BGP configuration into a runtime resource.
type BGPPeerConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *BGPPeerConfigController) Name() string {
	return "network.BGPPeerConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BGPPeerConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: configresource.NamespaceName,
			Type:      configresource.MachineConfigType,
			ID:        optional.Some(configresource.ActiveID),
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
func (ctrl *BGPPeerConfigController) Outputs() []controller.Output {
	return []controller.Output{{Type: network.BGPPeerConfigType, Kind: controller.OutputExclusive}}
}

// Run implements controller.Controller interface.
func (ctrl *BGPPeerConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *BGPPeerConfigController) reconcile(ctx context.Context, r controller.Runtime) error {
	machineConfig, err := safe.ReaderGetByID[*configresource.MachineConfig](ctx, r, configresource.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing link statuses: %w", err)
	}

	desired := renderBGPPeerConfig(machineConfig, network.NewLinkResolver(linkStatuses.All))

	current, err := safe.ReaderGetByID[*network.BGPPeerConfig](ctx, r, network.BGPPeerConfigID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting BGP peer config: %w", err)
	}

	return reconcileBGPPeerConfig(ctx, r, current, desired)
}

func renderBGPPeerConfig(machineConfig *configresource.MachineConfig, resolver *network.LinkResolver) *network.BGPPeerConfigSpec {
	if machineConfig == nil {
		return nil
	}

	cfg := machineConfig.Config().NetworkBGPPeerConfig()
	if cfg == nil {
		return nil
	}

	spec := buildBGPPeerConfigSpec(cfg, resolver)

	return &spec
}

func reconcileBGPPeerConfig(
	ctx context.Context,
	r controller.Runtime,
	current *network.BGPPeerConfig,
	desired *network.BGPPeerConfigSpec,
) error {
	if desired == nil {
		return destroyBGPPeerConfig(ctx, r, current)
	}

	if current != nil && reflect.DeepEqual(current.TypedSpec(), desired) {
		return nil
	}

	if err := safe.WriterModify(ctx, r, network.NewBGPPeerConfig(), func(res *network.BGPPeerConfig) error {
		*res.TypedSpec() = *desired

		return nil
	}); err != nil {
		return fmt.Errorf("error writing BGP peer config: %w", err)
	}

	return nil
}

func destroyBGPPeerConfig(ctx context.Context, r controller.Runtime, current *network.BGPPeerConfig) error {
	if current == nil {
		return nil
	}

	ready, err := r.Teardown(ctx, current.Metadata())
	if err != nil {
		return fmt.Errorf("error tearing down BGP peer config: %w", err)
	}

	if !ready {
		return nil
	}

	if err = r.Destroy(ctx, current.Metadata()); err != nil {
		return fmt.Errorf("error destroying BGP peer config: %w", err)
	}

	return nil
}

func buildBGPPeerConfigSpec(cfg talosconfig.NetworkBGPPeerConfig, resolver *network.LinkResolver) network.BGPPeerConfigSpec {
	spec := network.BGPPeerConfigSpec{
		LocalASN:       cfg.LocalASN(),
		RouterID:       cfg.RouterID(),
		RouteSource:    cfg.RouteSource(),
		AdvertiseLinks: make([]string, len(cfg.AdvertiseLinks())),
		Multipath:      cfg.Multipath(),
		MaxPaths:       cfg.MaxPaths(),
		Neighbors:      make([]network.BGPNeighborConfigSpec, 0, len(cfg.Neighbors())),
	}

	for i, link := range cfg.AdvertiseLinks() {
		spec.AdvertiseLinks[i] = resolver.Resolve(link)
	}

	for _, neighbor := range cfg.Neighbors() {
		neighborSpec := network.BGPNeighborConfigSpec{
			Address:  neighbor.Address(),
			Link:     resolver.Resolve(neighbor.Link()),
			PeerASN:  neighbor.PeerASN(),
			HoldTime: neighbor.HoldTime(),
		}

		if bfd := neighbor.BFD(); bfd != nil {
			neighborSpec.BFD = &network.BGPBFDConfigSpec{
				TransmitInterval: bfd.TransmitInterval(),
				ReceiveInterval:  bfd.ReceiveInterval(),
				DetectMultiplier: bfd.DetectMultiplier(),
			}
		}

		spec.Neighbors = append(spec.Neighbors, neighborSpec)
	}

	return spec
}
