// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// SeccompProfileController manages SeccompProfiles.
type SeccompProfileController struct{}

// Name implements controller.StatsController interface.
func (ctrl *SeccompProfileController) Name() string {
	return "cri.SeccompProfileController"
}

// Inputs implements controller.StatsController interface.
func (ctrl *SeccompProfileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.StatsController interface.
func (ctrl *SeccompProfileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cri.SeccompProfileType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.StatsController interface.
func (ctrl *SeccompProfileController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg.Config().Machine() != nil {
			for _, profile := range cfg.Config().Machine().SeccompProfiles() {
				if err = safe.WriterModify(ctx, r, cri.NewSeccompProfile(profile.Name()), func(cri *cri.SeccompProfile) error {
					cri.TypedSpec().Name = profile.Name()
					cri.TypedSpec().Value = profile.Value()

					return nil
				}); err != nil {
					return err
				}
			}
		}

		if err = safe.CleanupOutputs[*cri.SeccompProfile](ctx, r); err != nil {
			return err
		}
	}
}
