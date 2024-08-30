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

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MetaProvider wraps acquiring meta.
type MetaProvider interface {
	Meta() machineruntime.Meta
}

// DropUpgradeFallbackController removes upgrade fallback key once machine reaches ready & running.
type DropUpgradeFallbackController struct {
	MetaProvider MetaProvider
}

// Name implements controller.Controller interface.
func (ctrl *DropUpgradeFallbackController) Name() string {
	return "runtime.DropUpgradeFallbackController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DropUpgradeFallbackController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MachineStatusType,
			ID:        optional.Some(runtime.MachineStatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DropUpgradeFallbackController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *DropUpgradeFallbackController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineStatus, err := safe.ReaderGetByID[*runtime.MachineStatus](ctx, r, runtime.MachineStatusID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine status: %w", err)
		}

		if !(machineStatus.TypedSpec().Stage == runtime.MachineStageRunning && machineStatus.TypedSpec().Status.Ready) {
			continue
		}

		ok, err := ctrl.MetaProvider.Meta().DeleteTag(ctx, meta.Upgrade)
		if err != nil {
			return err
		}

		if ok {
			logger.Info("removing fallback entry")

			if err = ctrl.MetaProvider.Meta().Flush(); err != nil {
				return err
			}
		}

		// terminating the controller here, as removing fallback is required only once on boot after upgrade
		return nil
	}
}
