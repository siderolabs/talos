// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"context"
	"fmt"
	"log"

	"github.com/AlekSi/pointer"
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/resources/config"
)

// MachineTypeController manages config.MachineType based on configuration.
type MachineTypeController struct {
}

// Name implements controller.Controller interface.
func (ctrl *MachineTypeController) Name() string {
	return "config.MachineTypeController"
}

// ManagedResources implements controller.Controller interface.
func (ctrl *MachineTypeController) ManagedResources() (resource.Namespace, resource.Type) {
	return config.NamespaceName, config.MachineTypeType
}

// Run implements controller.Controller interface.
func (ctrl *MachineTypeController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if err := r.UpdateDependencies([]controller.Dependency{
		{
			Namespace: config.NamespaceName,
			Type:      config.V1Alpha1Type,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.DependencyWeak,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var machineType machine.Type

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.V1Alpha1Type, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			machineType = cfg.(*config.V1Alpha1).Config().Machine().Type()
		}

		if err = r.Update(ctx, config.NewMachineType(), func(r resource.Resource) error {
			r.(*config.MachineType).SetMachineType(machineType)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}
}
