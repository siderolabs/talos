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

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MachineStatusPublisherController watches MachineStatusPublishers, sets/resets kernel params.
type MachineStatusPublisherController struct {
	V1Alpha1Events v1alpha1runtime.Publisher
}

// Name implements controller.Controller interface.
func (ctrl *MachineStatusPublisherController) Name() string {
	return "runtime.MachineStatusPublisherController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MachineStatusPublisherController) Inputs() []controller.Input {
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
func (ctrl *MachineStatusPublisherController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *MachineStatusPublisherController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineStatus, err := safe.ReaderGet[*runtime.MachineStatus](ctx, r, runtime.NewMachineStatus().Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error reading machine status: %w", err)
		}

		ctrl.V1Alpha1Events.Publish(ctx, &machine.MachineStatusEvent{
			Stage: machine.MachineStatusEvent_MachineStage(machineStatus.TypedSpec().Stage),
			Status: &machine.MachineStatusEvent_MachineStatus{
				Ready: machineStatus.TypedSpec().Status.Ready,
				UnmetConditions: xslices.Map(machineStatus.TypedSpec().Status.UnmetConditions,
					func(unmetCondition runtime.UnmetCondition) *machine.MachineStatusEvent_MachineStatus_UnmetCondition {
						return &machine.MachineStatusEvent_MachineStatus_UnmetCondition{
							Name:   unmetCondition.Name,
							Reason: unmetCondition.Reason,
						}
					}),
			},
		})

		r.ResetRestartBackoff()
	}
}
