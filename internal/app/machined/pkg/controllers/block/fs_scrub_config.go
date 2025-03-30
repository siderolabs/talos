// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// FSScrubConfigController generates configuration for watchdog timers.
type FSScrubConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *FSScrubConfigController) Name() string {
	return "block.FSScrubConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FSScrubConfigController) Inputs() []controller.Input {
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
func (ctrl *FSScrubConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.FSScrubConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *FSScrubConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) (err error) {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg != nil {
			for _, conf := range cfg.Config().Runtime().FilesystemScrub() {
				if err := safe.WriterModify(ctx, r, block.NewFSScrubConfig(conf.Name()), func(res *block.FSScrubConfig) error {
					res.TypedSpec().Name = conf.Name()
					res.TypedSpec().Mountpoint = conf.Mountpoint()
					res.TypedSpec().Period = conf.Period()

					return nil
				}); err != nil {
					return fmt.Errorf("error updating filesystem scrub config: %w", err)
				}
			}
		}

		if err = safe.CleanupOutputs[*block.FSScrubConfig](ctx, r); err != nil {
			return err
		}
	}
}
