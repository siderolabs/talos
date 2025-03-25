// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// PCIDriverRebindConfigController generates configuration for PCI rebind.
type PCIDriverRebindConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *PCIDriverRebindConfigController) Name() string {
	return "hardware.PCIDriverRebindConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PCIDriverRebindConfigController) Inputs() []controller.Input {
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
func (ctrl *PCIDriverRebindConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.PCIDriverRebindConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *PCIDriverRebindConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) (err error) {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg != nil {
			for _, pciDriverRebindConfig := range cfg.Config().PCIDriverRebindConfig().PCIDriverRebindConfigs() {
				if err := safe.WriterModify(ctx, r, hardware.NewPCIDriverRebindConfig(pciDriverRebindConfig.PCIID()), func(res *hardware.PCIDriverRebindConfig) error {
					res.TypedSpec().PCIID = pciDriverRebindConfig.PCIID()
					res.TypedSpec().TargetDriver = pciDriverRebindConfig.TargetDriver()

					return nil
				}); err != nil {
					return fmt.Errorf("error updating PCI rebind config: %w", err)
				}
			}
		}

		if err = safe.CleanupOutputs[*hardware.PCIDriverRebindConfig](ctx, r); err != nil {
			return err
		}
	}
}
