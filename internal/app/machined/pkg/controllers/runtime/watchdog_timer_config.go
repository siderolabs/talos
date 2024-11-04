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

// WatchdogTimerConfigController generates configuration for watchdog timers.
type WatchdogTimerConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *WatchdogTimerConfigController) Name() string {
	return "runtime.WatchdogTimerConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WatchdogTimerConfigController) Inputs() []controller.Input {
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
func (ctrl *WatchdogTimerConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.WatchdogTimerConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *WatchdogTimerConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) (err error) {
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

		if cfg != nil {
			if watchdogConfig := cfg.Config().Runtime().WatchdogTimer(); watchdogConfig != nil {
				if err = safe.WriterModify(ctx, r, runtime.NewWatchdogTimerConfig(), func(cfg *runtime.WatchdogTimerConfig) error {
					cfg.TypedSpec().Device = watchdogConfig.Device()
					cfg.TypedSpec().Timeout = watchdogConfig.Timeout()

					return nil
				}); err != nil {
					return fmt.Errorf("error updating kmsg log config: %w", err)
				}
			}
		}

		if err = safe.CleanupOutputs[*runtime.WatchdogTimerConfig](ctx, r); err != nil {
			return err
		}
	}
}
