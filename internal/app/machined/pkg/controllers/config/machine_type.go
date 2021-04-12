// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"
	"fmt"
	"log"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/resources/config"
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
			ID:        pointer.ToString(config.V1Alpha1ID),
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
func (ctrl *MachineTypeController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var machineType machine.Type

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			machineType = cfg.(*config.MachineConfig).Config().Machine().Type()
		}

		if err = r.Modify(ctx, config.NewMachineType(), func(r resource.Resource) error {
			r.(*config.MachineType).SetMachineType(machineType)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}
}
