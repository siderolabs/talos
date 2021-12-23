// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

// KernelModuleConfigController watches v1alpha1.Config, creates/updates/deletes kernel module specs.
type KernelModuleConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *KernelModuleConfigController) Name() string {
	return "runtime.KernelModuleConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KernelModuleConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KernelModuleConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.KernelModuleSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KernelModuleConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting config: %w", err)
				}
			}

			touchedIDs := make(map[resource.ID]struct{})

			if cfg != nil {
				c, _ := cfg.(*config.MachineConfig) //nolint:errcheck
				for _, module := range c.Config().Machine().Kernel().Modules() {
					touchedIDs[module.Name()] = struct{}{}

					item := runtime.NewKernelModuleSpec(runtime.NamespaceName, module.Name())

					if err = r.Modify(ctx, item, func(res resource.Resource) error {
						res.(*runtime.KernelModuleSpec).TypedSpec().Name = module.Name()

						return nil
					}); err != nil {
						return err
					}
				}
			}

			// list keys for cleanup
			list, err := r.List(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelModuleSpecType, "", resource.VersionUndefined))
			if err != nil {
				return fmt.Errorf("error listing resources: %w", err)
			}

			for _, res := range list.Items {
				if res.Metadata().Owner() != ctrl.Name() {
					continue
				}

				if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
					if err = r.Destroy(ctx, res.Metadata()); err != nil {
						return fmt.Errorf("error cleaning up specs: %w", err)
					}
				}
			}
		}
	}
}
