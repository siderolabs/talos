// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// CustomizationConfigController projects CRI customizations from the machine config.
type CustomizationConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *CustomizationConfigController) Name() string {
	return "cri.CustomizationConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CustomizationConfigController) Inputs() []controller.Input {
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
func (ctrl *CustomizationConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cri.CustomizationConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CustomizationConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get machine config: %w", err)
		}

		if cfg != nil {
			for _, customization := range cfg.Config().CRICustomizationConfigs() {
				if err = safe.WriterModify(ctx, r, cri.NewCustomizationConfig(customization.Name()), func(res *cri.CustomizationConfig) error {
					res.TypedSpec().Content = customization.Content()

					return nil
				}); err != nil {
					return fmt.Errorf("failed to write customization config %q: %w", customization.Name(), err)
				}
			}
		}

		if err := safe.CleanupOutputs[*cri.CustomizationConfig](ctx, r); err != nil {
			return fmt.Errorf("failed to clean up outputs: %w", err)
		}
	}
}
