// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// DevicesStatusController loads extensions.yaml and updates DevicesStatus resources.
type DevicesStatusController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *DevicesStatusController) Name() string {
	return "runtime.DevicesStatusController"
}

// Inputs implements controller.Controller interface.
func (ctl *DevicesStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DevicesStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.DevicesStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *DevicesStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// in container mode, devices are always ready
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return safe.WriterModify(ctx, r, runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID), func(status *runtime.DevicesStatus) error {
			status.TypedSpec().Ready = true

			return nil
		})
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		service, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "udevd")
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !(service.TypedSpec().Running && service.TypedSpec().Healthy) {
			// condition not met
			continue
		}

		if err := safe.WriterModify(ctx, r, runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID), func(status *runtime.DevicesStatus) error {
			status.TypedSpec().Ready = true

			return nil
		}); err != nil {
			return err
		}

		// everything is done, ready, stop the controller
		return nil
	}
}
