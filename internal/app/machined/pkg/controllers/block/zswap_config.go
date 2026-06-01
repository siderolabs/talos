// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ZswapConfigController provides zswap configuration based machine configuration.
type ZswapConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ZswapConfigController) Name() string {
	return "block.ZswapConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ZswapConfigController) Inputs() []controller.Input {
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
func (ctrl *ZswapConfigController) Outputs() []controller.Output {
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
func (ctrl *ZswapConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		// load config if present
		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching machine configuration")
		}

		r.StartTrackingOutputs()

		var zswapCfg configconfig.ZswapConfig

		if cfg != nil {
			zswapCfg = cfg.Config().ZswapConfig()
		}

		if zswapCfg != nil { // enabled
			if err := safe.WriterModify(ctx, r, runtime.NewKernelParamSpec(runtime.NamespaceName, "sys.module.zswap.parameters.enabled"),
				func(p *runtime.KernelParamSpec) error {
					p.TypedSpec().Value = "Y"

					return nil
				}); err != nil {
				return fmt.Errorf("error setting zswap config: %w", err)
			}

			if err := safe.WriterModify(ctx, r, runtime.NewKernelParamSpec(runtime.NamespaceName, "sys.module.zswap.parameters.max_pool_percent"),
				func(p *runtime.KernelParamSpec) error {
					p.TypedSpec().Value = strconv.Itoa(zswapCfg.MaxPoolPercent())

					return nil
				}); err != nil {
				return fmt.Errorf("error setting zswap config: %w", err)
			}

			if err := safe.WriterModify(ctx, r, runtime.NewKernelParamSpec(runtime.NamespaceName, "sys.module.zswap.parameters.shrinker_enabled"),
				func(p *runtime.KernelParamSpec) error {
					if zswapCfg.ShrinkerEnabled() {
						p.TypedSpec().Value = "Y"
					} else {
						p.TypedSpec().Value = "N"
					}

					return nil
				}); err != nil {
				return fmt.Errorf("error setting zswap config: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*runtime.KernelParamSpec](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up volume configuration: %w", err)
		}
	}
}
