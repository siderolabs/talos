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
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	configtypes "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// WifiConfigController manages network.WifiSpec based on machine configuration.
type WifiConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *WifiConfigController) Name() string {
	return "network.WifiConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WifiConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *WifiConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.WifiSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *WifiConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error reading machine configuration: %w", err)
		}

		if cfg != nil {
			if err = ctrl.apply(ctx, r, cfg.Config().NetworkWifiConfigs()); err != nil {
				return fmt.Errorf("error applying WifiSpec: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*network.WifiSpec](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up WifiSpec: %w", err)
		}
	}
}

func (ctrl *WifiConfigController) apply(ctx context.Context, r controller.Runtime, configs []configtypes.NetworkWifiConfig) error {
	for _, cfg := range configs {
		if err := safe.WriterModify(ctx, r, network.NewWifiSpec(network.NamespaceName, cfg.Name()), func(spec *network.WifiSpec) error {
			spec.TypedSpec().CountryCode = cfg.CountryCode()
			spec.TypedSpec().Networks = xslices.Map(cfg.Networks(), func(n configtypes.WifiNetwork) network.WifiNetwork {
				return network.WifiNetwork{
					SSID:   n.SSID(),
					PSK:    n.PSK(),
					Hidden: n.Hidden(),
				}
			})

			return nil
		}); err != nil {
			return fmt.Errorf("error writing WifiSpec: %w", err)
		}
	}

	return nil
}
