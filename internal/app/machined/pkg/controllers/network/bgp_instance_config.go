// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// BGPInstanceConfigController projects machine BGP documents into desired-state resources.
//
// Runtime link and address resolution belongs to the operational consumers of these resources.
type BGPInstanceConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *BGPInstanceConfigController) Name() string {
	return "network.BGPInstanceConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BGPInstanceConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: configresource.NamespaceName,
			Type:      configresource.MachineConfigType,
			ID:        optional.Some(configresource.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *BGPInstanceConfigController) Outputs() []controller.Output {
	return []controller.Output{{Type: network.BGPInstanceConfigType, Kind: controller.OutputExclusive}}
}

// Run implements controller.Controller interface.
func (ctrl *BGPInstanceConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

func (ctrl *BGPInstanceConfigController) reconcile(ctx context.Context, r controller.Runtime) error {
	machineConfig, err := safe.ReaderGetByID[*configresource.MachineConfig](ctx, r, configresource.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	desired, err := renderBGPInstanceConfigs(machineConfig)
	if err != nil {
		return fmt.Errorf("error rendering BGP instance configs: %w", err)
	}

	r.StartTrackingOutputs()

	for name, spec := range desired {
		if err = safe.WriterModify(ctx, r, network.NewBGPInstanceConfig(name), func(res *network.BGPInstanceConfig) error {
			*res.TypedSpec() = spec

			return nil
		}); err != nil {
			return fmt.Errorf("error writing BGP instance config %q: %w", name, err)
		}
	}

	if err = safe.CleanupOutputs[*network.BGPInstanceConfig](ctx, r); err != nil {
		return fmt.Errorf("error cleaning up BGP instance configs: %w", err)
	}

	return nil
}

func renderBGPInstanceConfigs(machineConfig *configresource.MachineConfig) (map[string]network.BGPInstanceConfigSpec, error) {
	desired := map[string]network.BGPInstanceConfigSpec{}

	if machineConfig == nil {
		return desired, nil
	}

	cfg := machineConfig.Config()

	projectionState := newBGPProjectionState(cfg)

	for _, instance := range cfg.NetworkBGPInstanceConfigs() {
		name := instance.Name()
		if _, exists := desired[name]; exists {
			return nil, fmt.Errorf("duplicate BGP instance config %q", name)
		}

		spec, err := renderBGPInstanceConfig(instance, projectionState)
		if err != nil {
			return nil, fmt.Errorf("BGP instance %q: %w", name, err)
		}

		desired[name] = spec
	}

	return desired, nil
}

type bgpProjectionState struct {
	vrfs        map[string]talosconfig.NetworkVRFConfig
	usedVRFs    map[string]string
	defaultName string
}

func newBGPProjectionState(cfg talosconfig.Config) *bgpProjectionState {
	state := &bgpProjectionState{
		vrfs:     map[string]talosconfig.NetworkVRFConfig{},
		usedVRFs: map[string]string{},
	}

	for _, linkConfig := range cfg.NetworkCommonLinkConfigs() {
		if vrf, ok := linkConfig.(talosconfig.NetworkVRFConfig); ok {
			state.vrfs[vrf.Name()] = vrf
		}
	}

	return state
}

func renderBGPInstanceConfig(
	instance talosconfig.NetworkBGPInstanceConfig,
	state *bgpProjectionState,
) (network.BGPInstanceConfigSpec, error) {
	name := instance.Name()

	vrfName, vrfTable, err := resolveBGPInstanceDomain(name, instance.VRF(), state)
	if err != nil {
		return network.BGPInstanceConfigSpec{}, err
	}

	spec := buildBGPInstanceConfigSpec(instance)

	spec.VRF = vrfName
	spec.VRFTable = vrfTable

	return spec, nil
}

func resolveBGPInstanceDomain(
	name string,
	configuredVRF string,
	state *bgpProjectionState,
) (string, nethelpers.RoutingTable, error) {
	vrfName := configuredVRF
	if vrfName == "" {
		if state.defaultName != "" {
			return "", 0, fmt.Errorf("instances %q and %q both use the default routing domain", state.defaultName, name)
		}

		state.defaultName = name

		return "", nethelpers.TableMain, nil
	}

	if other, exists := state.usedVRFs[vrfName]; exists {
		return "", 0, fmt.Errorf("instances %q and %q both use VRF %q", other, name, vrfName)
	}

	vrfConfig, exists := state.vrfs[configuredVRF]
	if !exists {
		return "", 0, fmt.Errorf("references missing VRFConfig %q", configuredVRF)
	}

	state.usedVRFs[vrfName] = name

	return vrfName, vrfConfig.Table(), nil
}

func buildBGPInstanceConfigSpec(cfg talosconfig.NetworkBGPInstanceConfig) network.BGPInstanceConfigSpec {
	spec := network.BGPInstanceConfigSpec{
		LocalASN:       cfg.LocalASN(),
		RouterID:       cfg.RouterID(),
		RouteSource:    cfg.RouteSource(),
		AdvertiseLinks: make([]string, len(cfg.AdvertiseLinks())),
		Multipath:      cfg.Multipath(),
		MaxPaths:       cfg.MaxPaths(),
		Neighbors:      make([]network.BGPNeighborConfigSpec, 0, len(cfg.Neighbors())),
	}

	for i, link := range cfg.AdvertiseLinks() {
		spec.AdvertiseLinks[i] = link
	}

	for _, neighbor := range cfg.Neighbors() {
		neighborSpec := network.BGPNeighborConfigSpec{
			Address:  neighbor.Address(),
			Link:     neighbor.Link(),
			PeerASN:  neighbor.PeerASN(),
			LocalASN: neighbor.LocalASN(),
			Passive:  neighbor.Passive(),
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
