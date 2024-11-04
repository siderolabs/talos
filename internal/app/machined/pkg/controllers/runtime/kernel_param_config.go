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

	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// KernelParamConfigController watches v1alpha1.Config, creates/updates/deletes kernel param specs.
type KernelParamConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *KernelParamConfigController) Name() string {
	return "runtime.KernelParamConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KernelParamConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KernelParamConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.KernelParamSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KernelParamConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

		setKernelParam := func(kind, key, value string) error {
			item := runtime.NewKernelParamSpec(runtime.NamespaceName, kind+"."+key)

			return safe.WriterModify(ctx, r, item, func(res *runtime.KernelParamSpec) error {
				res.TypedSpec().Value = value

				return nil
			})
		}

		if cfg != nil && cfg.Config().Machine() != nil {
			for key, value := range cfg.Config().Machine().Sysctls() {
				if err = setKernelParam(kernel.Sysctl, key, value); err != nil {
					return err
				}
			}

			for key, value := range cfg.Config().Machine().Sysfs() {
				if err = setKernelParam(kernel.Sysfs, key, value); err != nil {
					return err
				}
			}
		}

		if err = safe.CleanupOutputs[*runtime.KernelParamSpec](ctx, r); err != nil {
			return err
		}
	}
}
