// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
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
			ID:        pointer.To(config.V1Alpha1ID),
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
func (ctrl *KernelParamConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

			setKernelParam := func(kind, key, value string) error {
				item := runtime.NewKernelParamSpec(runtime.NamespaceName, strings.Join([]string{kind, key}, "."))

				touchedIDs[item.Metadata().ID()] = struct{}{}

				return r.Modify(ctx, item, func(res resource.Resource) error {
					res.(*runtime.KernelParamSpec).TypedSpec().Value = value

					return nil
				})
			}

			if cfg != nil {
				c, _ := cfg.(*config.MachineConfig) //nolint:errcheck
				for key, value := range c.Config().Machine().Sysctls() {
					if err = setKernelParam(kernel.Sysctl, key, value); err != nil {
						return err
					}
				}

				for key, value := range c.Config().Machine().Sysfs() {
					if err = setKernelParam(kernel.Sysfs, key, value); err != nil {
						return err
					}
				}
			}

			// list keys for cleanup
			list, err := r.List(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelParamSpecType, "", resource.VersionUndefined))
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

		r.ResetRestartBackoff()
	}
}
