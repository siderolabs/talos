// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
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
		// Facilitates dependsOn.containers
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

		wakeCtrlAfter = minOptionalDuration(wakeCtrlAfter, wakeAfter)
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

	containerInstanceStatus, err := ctrl.latestInstanceStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	imageStatus, err := safe.ReaderGetByID[*containers.ContainerImageStatus](ctx, runtime, containerSpecID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return optional.None[time.Duration](), fmt.Errorf("failed to get image status %q: %w", containerSpecID, err)
		}

		imageStatus = nil
	}

	// waitingFor gates only affect the status before the container instance is created.
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

	instanceSpec, err := ctrl.CurrentInstanceSpec(ctx, runtime, containerSpecID, containerInstanceStatus)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	// The digest which the current reference resolves to, which is not necessarily the one actually running.
	digest, err := containers.GetImageDigest(ctx, runtime, containerSpecID, containerSpec.TypedSpec().Image.Ref)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to resolve image digest %q: %w", containerSpecID, err)
	}

	var before, after containers.ContainerStatusSpec

	if err := safe.WriterModify(ctx, runtime,
		containers.NewContainerStatus(containers.NamespaceName, containerSpecID),
		func(res *containers.ContainerStatus) error {
			before = *res.TypedSpec()

			assembleStatus(res.TypedSpec(), containerSpec, imageStatus, containerInstanceStatus, instanceSpec, digest, waitingFor, before)

			after = *res.TypedSpec()

			return nil
		},
	); err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to write container status %q: %w", containerSpecID, err)
	}

	// This is the one place with a before-and-after view of the aggregate, so it is where a state
	// change is worth a line in the log rather than in every controller that causes one.
	logTransition(logger, containerSpecID, before, after)

	return minOptionalDuration(wakeAfter, restartWindowWakeAfter(containerInstanceStatus)), nil
}

// latestInstanceStatus returns the newest instance status of containerSpecID, or nil if there is none.
func (ctrl *StatusController) latestInstanceStatus(
	ctx context.Context,
	reader controller.Reader,
	containerSpecID string,
) (*containers.ContainerInstanceStatus, error) {
	instanceStatuses, err := safe.ReaderListAll[*containers.ContainerInstanceStatus](ctx, reader,
		state.WithIDQuery(containers.InstanceIDQuery(containerSpecID)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance statuses %q: %w", containerSpecID, err)
	}

	var latest *containers.ContainerInstanceStatus

	for instanceStatus := range instanceStatuses.All() {
		if latest == nil || instanceStatus.TypedSpec().Generation > latest.TypedSpec().Generation {
			latest = instanceStatus
		}
	}

	return latest, nil
}

// CurrentInstanceSpec returns the spec of containerID's current instance, or nil if there is none.
func (ctrl *StatusController) CurrentInstanceSpec(
	ctx context.Context,
	reader controller.Reader,
	containerSpecID string,
	instanceStatus *containers.ContainerInstanceStatus,
) (*containers.ContainerInstanceSpec, error) {
	if instanceStatus == nil {
		return nil, nil
	}

	instanceSpecID := containers.InstanceID(containerSpecID, instanceStatus.TypedSpec().Generation)

	instanceSpec, err := safe.ReaderGetByID[*containers.ContainerInstanceSpec](ctx, reader, instanceSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get instance spec %q: %w", instanceSpecID, err)
	}

	return instanceSpec, nil
}

// logTransition reports a change in the aggregated state or error of one container.
func logTransition(logger *zap.Logger, containerID string, before, after containers.ContainerStatusSpec) {
	if before.State != after.State {
		fields := []zap.Field{
			zap.String("container", containerID),
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
			zap.String("container", containerID),
			zap.Stringer("state", after.State),
			zap.String("error", after.Error),
		)
	}
}

// assembleStatus derives the aggregated status for one container from its already-resolved inputs.
func assembleStatus(
	containerStatusSpec *containers.ContainerStatusSpec,
	containerSpec *containers.ContainerSpec,
	imageStatus *containers.ContainerImageStatus,
	instanceStatus *containers.ContainerInstanceStatus,
	instanceSpec *containers.ContainerInstanceSpec,
	imageDigest string,
	waitingFor []string,
	prevContainerStatusSpec containers.ContainerStatusSpec,
) {
	isStopping := instanceSpec != nil && instanceSpec.Metadata().Phase() == resource.PhaseTearingDown

	containerStatusSpec.Image = resolveReportedImage(containerSpec, instanceSpec, imageDigest)

	containerStatusSpec.PID = 0
	containerStatusSpec.WaitingFor = nil

	// There is no instance status between generations, nor for as long as a restart waits on a gate.
	// The last execution's outcome is what an operator is looking at in exactly those windows, so it
	// survives the gap rather than reading back as a container that never crashed.
	containerStatusSpec.RestartCount = prevContainerStatusSpec.RestartCount
	containerStatusSpec.ExitCode = prevContainerStatusSpec.ExitCode

	if instanceStatus != nil {
		containerStatusSpec.RestartCount = instanceStatus.TypedSpec().Generation

		containerStatusSpec.ExitCode = instanceStatus.TypedSpec().ExitCode
		if instanceStatus.TypedSpec().Phase == containers.ContainerInstancePhaseRunning {
			containerStatusSpec.PID = instanceStatus.TypedSpec().PID
		}
	}

	containerStatusSpec.State = resolveContainerState(instanceStatus, imageStatus, len(waitingFor) == 0, isStopping)

	if containerStatusSpec.State == containers.ContainerStateStopping {
		containerStatusSpec.Health = prevContainerStatusSpec.Health
	} else {
		containerStatusSpec.Health = containerStatusSpec.State.Health()
	}

	if containerStatusSpec.State == containers.ContainerStatePending {
		containerStatusSpec.WaitingFor = waitingFor
	}

	containerStatusSpec.Error = deriveReportedError(instanceStatus, imageStatus)

	// Same reasoning as RestartCount and ExitCode: keep the reason the last execution ended visible
	// while the status that reported it is gone.
	if containerStatusSpec.Error == "" && instanceStatus == nil {
		containerStatusSpec.Error = prevContainerStatusSpec.Error
	}
}

// resolveReportedImage picks the image to report.
func resolveReportedImage(containerSpec *containers.ContainerSpec, containerInstanceSpec *containers.ContainerInstanceSpec, imageDigest string) string {
	if containerInstanceSpec != nil && containerInstanceSpec.TypedSpec().Image != "" {
		return containerInstanceSpec.TypedSpec().Image
	}

	if imageDigest != "" {
		return imageDigest
	}

	return containerSpec.TypedSpec().Image.Ref
}

func deriveReportedError(instanceStatus *containers.ContainerInstanceStatus, imageStatus *containers.ContainerImageStatus) string {
	if instanceStatus != nil && instanceStatus.TypedSpec().Error != "" {
		return instanceStatus.TypedSpec().Error
	}

	if imageStatus != nil && imageStatus.TypedSpec().Error != "" {
		return imageStatus.TypedSpec().Error
	}

	return ""
}

// resolveContainerState maps the observable resources onto a container state.
//
// There is no terminal state: a finished instance means a restart is pending, which is exited while
// it is still fresh and backoff once RestartInterval has elapsed waiting for it.
func resolveContainerState(
	instanceStatus *containers.ContainerInstanceStatus,
	imageStatus *containers.ContainerImageStatus,
	gatesReady bool,
	isStopping bool,
) containers.ContainerState {
	if instanceStatus != nil {
		if isStopping {
			return containers.ContainerStateStopping
		}

		return instanceState(instanceStatus)
	}

	// Only the phases that say something the gate check cannot: an image still coming down, or one
	// that will not. Pending and Ready are left to the gate check below, which reports "image" as
	// unmet for as long as there is no usable digest.
	if imageStatus != nil {
		switch imageStatus.TypedSpec().Phase {
		case containers.ContainerImagePhasePulling:
			return containers.ContainerStatePulling
		case containers.ContainerImagePhaseFailed:
			return containers.ContainerStateBackoff
		case containers.ContainerImagePhasePending, containers.ContainerImagePhaseReady:
		}
	}

	if !gatesReady {
		return containers.ContainerStatePending
	}

	return containers.ContainerStateStarting
}

// restartWindowWakeAfter returns how long is left of the window that makes a finished instance
// Exited rather than Backoff, if it is still inside it.
//
// That transition is the one this controller makes on its own clock rather than in response to
// another resource, so it is also the one it has to schedule for itself: a finished instance that
// stays in place — its replacement blocked on a gate, say — has nothing else coming that would
// trigger the pass.
func restartWindowWakeAfter(instance *containers.ContainerInstanceStatus) optional.Optional[time.Duration] {
	if instance == nil {
		return optional.None[time.Duration]()
	}

	switch instance.TypedSpec().Phase {
	case containers.ContainerInstancePhaseTerminated, containers.ContainerInstancePhaseFailed:
		if remaining := RestartInterval - time.Since(instance.TypedSpec().FinishedAt); remaining > 0 {
			return optional.Some(remaining)
		}
	case containers.ContainerInstancePhaseCreated, containers.ContainerInstancePhaseRunning:
	}

	return optional.None[time.Duration]()
}

func instanceState(instance *containers.ContainerInstanceStatus) containers.ContainerState {
	switch instance.TypedSpec().Phase {
	case containers.ContainerInstancePhaseCreated:
		return containers.ContainerStateStarting
	case containers.ContainerInstancePhaseRunning:
		return containers.ContainerStateRunning
	case containers.ContainerInstancePhaseTerminated, containers.ContainerInstancePhaseFailed:
		if time.Since(instance.TypedSpec().FinishedAt) < RestartInterval {
			return containers.ContainerStateExited
		}

		return containers.ContainerStateBackoff
	}

	return containers.ContainerStateStarting
}
