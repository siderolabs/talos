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
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	extconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ExtensionServicesConfigController watches v1alpha1.Config, creates/updates/deletes extension services config.
type ExtensionServicesConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ExtensionServicesConfigController) Name() string {
	return "runtime.ExtensionServicesConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ExtensionServicesConfigController) Inputs() []controller.Input {
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
func (ctrl *ExtensionServicesConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.ExtensionServicesConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ExtensionServicesConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg != nil && cfg.Config() != nil && cfg.Config().ExtensionServicesConfig() != nil {
			for _, ext := range cfg.Config().ExtensionServicesConfig().ConfigData() {
				if err = safe.WriterModify(ctx, r, runtime.NewExtensionServicesConfigSpec(runtime.NamespaceName, ext.Name()), func(spec *runtime.ExtensionServicesConfig) error {
					spec.TypedSpec().Files = xslices.Map(ext.ConfigFiles(), func(c extconfig.ExtensionServicesConfigFile) runtime.ExtensionServicesConfigFile {
						return runtime.ExtensionServicesConfigFile{
							Content:   c.Content(),
							MountPath: c.Path(),
						}
					})

					return nil
				}); err != nil {
					return err
				}
			}
		}

		if err = safe.CleanupOutputs[*runtime.ExtensionServicesConfig](ctx, r); err != nil {
			return err
		}
	}
}
