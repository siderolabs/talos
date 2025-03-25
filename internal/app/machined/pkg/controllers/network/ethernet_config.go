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

	configtypes "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EthernetConfigController manages network.EthernetSpec based on machine configuration.
type EthernetConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *EthernetConfigController) Name() string {
	return "network.EthernetConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EthernetConfigController) Inputs() []controller.Input {
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
func (ctrl *EthernetConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.EthernetSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *EthernetConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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
			if err = ctrl.apply(ctx, r, cfg.Config().EthernetConfigs()); err != nil {
				return fmt.Errorf("error applying EthernetSpec: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*network.EthernetSpec](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up EthernetSpec: %w", err)
		}
	}
}

func (ctrl *EthernetConfigController) apply(ctx context.Context, r controller.Runtime, configs []configtypes.EthernetConfig) error {
	for _, cfg := range configs {
		if err := safe.WriterModify(ctx, r, network.NewEthernetSpec(network.NamespaceName, cfg.Name()), func(spec *network.EthernetSpec) error {
			spec.TypedSpec().Rings = network.EthernetRingsSpec(cfg.Rings())
			spec.TypedSpec().Channels = network.EthernetChannelsSpec(cfg.Channels())
			spec.TypedSpec().Features = cfg.Features()

			return nil
		}); err != nil {
			return fmt.Errorf("error writing EthernetSpec: %w", err)
		}
	}

	return nil
}
