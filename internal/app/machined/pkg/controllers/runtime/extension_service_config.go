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

// ExtensionServiceConfigController watches v1alpha1.Config, creates/updates/deletes extension services config.
type ExtensionServiceConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ExtensionServiceConfigController) Name() string {
	return "runtime.ExtensionServiceConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ExtensionServiceConfigController) Inputs() []controller.Input {
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
func (ctrl *ExtensionServiceConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.ExtensionServiceConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ExtensionServiceConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

		if cfg != nil && cfg.Config() != nil {
			for _, extConfig := range cfg.Config().ExtensionServiceConfigs() {
				if err = safe.WriterModify(ctx, r, runtime.NewExtensionServiceConfigSpec(runtime.NamespaceName, extConfig.Name()), func(spec *runtime.ExtensionServiceConfig) error {
					spec.TypedSpec().Files = xslices.Map(extConfig.ConfigFiles(), func(c extconfig.ExtensionServiceConfigFile) runtime.ExtensionServiceConfigFile {
						return runtime.ExtensionServiceConfigFile{
							Content:   c.Content(),
							MountPath: c.MountPath(),
						}
					})

					spec.TypedSpec().Environment = extConfig.Environment()

					return nil
				}); err != nil {
					return err
				}
			}
		}

		if err = safe.CleanupOutputs[*runtime.ExtensionServiceConfig](ctx, r); err != nil {
			return err
		}
	}
}
