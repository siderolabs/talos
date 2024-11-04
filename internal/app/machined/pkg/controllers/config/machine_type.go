// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// MachineTypeController manages config.MachineType based on configuration.
type MachineTypeController struct{}

// Name implements controller.Controller interface.
func (ctrl *MachineTypeController) Name() string {
	return "config.MachineTypeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineTypeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MachineTypeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: config.MachineTypeType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MachineTypeController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var machineType machine.Type

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else if cfg.Config().Machine() != nil {
			machineType = cfg.Config().Machine().Type()
		}

		if err = safe.WriterModify(ctx, r, config.NewMachineType(), func(r *config.MachineType) error {
			r.SetMachineType(machineType)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
