// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
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
			ID:        optional.Some(config.V1Alpha1ID),
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
func (ctrl *KernelModuleConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		r.StartTrackingOutputs()

		if cfg != nil && cfg.Config().Machine() != nil {
			for _, module := range cfg.Config().Machine().Kernel().Modules() {
				item := runtime.NewKernelModuleSpec(runtime.NamespaceName, module.Name())

				if err = safe.WriterModify(ctx, r, item, func(res *runtime.KernelModuleSpec) error {
					res.TypedSpec().Name = module.Name()
					res.TypedSpec().Parameters = module.Parameters()

					return nil
				}); err != nil {
					return err
				}
			}
		}

		if err = safe.CleanupOutputs[*runtime.KernelModuleSpec](ctx, r); err != nil {
			return err
		}
	}
}
