// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// StatusController aggregates the per-stage statuses into the user-facing ContainerStatus.
type StatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *StatusController) Name() string {
	return "containers.StatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StatusController) Inputs() []controller.Input {
	return append(containerCreationGateInputs(),
		// dependsOn.containers.
		controller.Input{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
		controller.Input{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputWeak,
		},
	)
}

// Outputs implements controller.Controller interface.
func (ctrl *StatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *StatusController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	return runWithWakeTimer(ctx, runtime, func(ctx context.Context, runtime controller.Runtime) (optional.Optional[time.Duration], error) {
		wakeAfter, err := ctrl.reconcile(ctx, runtime, logger)
		if err != nil {
			logger.Error("failed to aggregate container statuses", zap.Error(err))
		}

		return wakeAfter, err
	})
}

// reconcile calculates and writes up-to-date ContainerStatus resources for all containers.
//
// returns duration until the next controller wake up (if any).
func (ctrl *StatusController) reconcile(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) (optional.Optional[time.Duration], error) {
	runtime.StartTrackingOutputs()

	containerSpecs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, runtime)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to list container specs: %w", err)
	}

	var wakeCtrlAfter optional.Optional[time.Duration]

	for containerSpec := range containerSpecs.All() {
		wakeAfter, err := ctrl.reconcileContainerStatus(ctx, runtime, logger, containerSpec)
		if err != nil {
			return optional.None[time.Duration](), err
		}

		wakeCtrlAfter = containers.EarliestWakeUpAfter(wakeCtrlAfter, wakeAfter)
	}

	if err := safe.CleanupOutputs[*containers.ContainerStatus](ctx, runtime); err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return wakeCtrlAfter, nil
}

func (ctrl *StatusController) reconcileContainerStatus(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpec *containers.ContainerSpec,
) (optional.Optional[time.Duration], error) {
	containerSpecID := containerSpec.Metadata().ID()

	containerInstanceStatus, err := containers.LatestInstanceStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	containerImageStatus, err := containerSpec.TypedSpec().CurrentImageStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	// waitingFor gates only affect the status before the container instance is created: once an
	// instance exists its own phase is the state, and rechecking the gates would keep this controller
	// polling dependsOn.paths at 1 Hz for the life of the container.
	var (
		waitingFor []string
		wakeAfter  optional.Optional[time.Duration]
	)

	if containerInstanceStatus == nil {
		waitingFor, wakeAfter, err = containerSpec.TypedSpec().Ready(ctx, runtime, containerSpecID)
		if err != nil {
			return optional.None[time.Duration](), fmt.Errorf("failed to check container ready %q: %w", containerSpecID, err)
		}

		if len(waitingFor) == 0 {
			wakeAfter = optional.None[time.Duration]()
		}
	}

	containerInstanceSpec, err := containers.CurrentInstanceSpec(ctx, runtime, containerSpecID, containerInstanceStatus)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	// The digest which the current reference resolves to, which is not necessarily the one actually running.
	imageDigest := containers.ImageDigest(containerImageStatus, containerSpec.TypedSpec().Image.Ref)

	var before, after containers.ContainerStatusSpec

	if err := safe.WriterModify(ctx, runtime,
		containers.NewContainerStatus(containers.NamespaceName, containerSpecID),
		func(res *containers.ContainerStatus) error {
			before = *res.TypedSpec()

			res.TypedSpec().Update(containerSpec, containerImageStatus, containerInstanceStatus, containerInstanceSpec, imageDigest, waitingFor, RestartInterval)

			after = *res.TypedSpec()

			return nil
		},
	); err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to write container status %q: %w", containerSpecID, err)
	}

	// This is the one place with a before-and-after view of the aggregate, so it is where a state
	// change is worth a line in the log rather than in every controller that causes one.
	logContainerStatusTransition(logger, containerSpecID, before, after)

	return containers.EarliestWakeUpAfter(wakeAfter, restartWindowWakeAfter(containerInstanceStatus)), nil
}

// restartWindowWakeAfter returns how long is left of the window that makes a finished instance
// Exited rather than Backoff, if it is still inside it.
//
// That transition is the one this controller makes on its own clock rather than in response to
// another resource, so it is also the one it has to schedule for itself: a finished instance that
// stays in place — its replacement blocked on a gate, say — has nothing else coming that would
// trigger the pass.
func restartWindowWakeAfter(containerInstanceStatus *containers.ContainerInstanceStatus) optional.Optional[time.Duration] {
	if containerInstanceStatus == nil {
		return optional.None[time.Duration]()
	}

	return containerInstanceStatus.TypedSpec().RestartWindowWakeAfter(RestartInterval)
}

// logContainerStatusTransition reports a change in the aggregated state or error of one container.
func logContainerStatusTransition(logger *zap.Logger, containerSpecID string, before, after containers.ContainerStatusSpec) {
	if before.State != after.State {
		fields := []zap.Field{
			zap.String("container", containerSpecID),
			zap.Stringer("from", before.State),
			zap.Stringer("to", after.State),
			zap.Stringer("health", after.Health),
		}

		if len(after.WaitingFor) > 0 {
			fields = append(fields, zap.Strings("waitingFor", after.WaitingFor))
		}

		if after.PID != 0 {
			fields = append(fields, zap.Uint32("pid", after.PID))
		}

		if after.Error != "" {
			fields = append(fields, zap.String("error", after.Error))
		}

		logger.Info("container state changed", fields...)

		return
	}

	// A container that stays in the same state but picks up a new error has still made news.
	if before.Error != after.Error && after.Error != "" {
		logger.Warn("container reported an error",
			zap.String("container", containerSpecID),
			zap.Stringer("state", after.State),
			zap.String("error", after.Error),
		)
	}
}
