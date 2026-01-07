// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// EnvironmentController watches v1alpha1.Config and sets environment variables accordingly.
type EnvironmentController struct{}

// Name implements controller.Controller interface.
func (ctrl *EnvironmentController) Name() string {
	return "runtime.EnvironmentController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EnvironmentController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EnvironmentController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.EnvironmentType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EnvironmentController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		r.StartTrackingOutputs()

		if cfg != nil && cfg.Config().Environment() != nil {
			for key, value := range cfg.Config().Environment().Variables() {
				if err := os.Setenv(key, value); err != nil {
					return fmt.Errorf("error setting env var: \"%s=%s\": %w", key, value, err)
				}
			}
		}

		item := runtime.NewEnvironment("machined")

		if err = safe.WriterModify(ctx, r, item, func(res *runtime.Environment) error {
			env := os.Environ()

			slices.Sort(env)

			res.TypedSpec().Variables = env

			return nil
		}); err != nil {
			return err
		}

		if err = safe.CleanupOutputs[*runtime.Environment](ctx, r); err != nil {
			return err
		}
	}
}
