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
	// The ContainerStatus entry containerCreationGateInputs contributes is self-referential here:
	// dependsOn.containers gates on another container's Health, computed by this same controller.
	// Reads of an own Output are permitted without an Input declaration, but the declaration is what
	// makes it reactive: one container's status write is what schedules the pass that lets a
	// dependent container notice, converging to a fixed point rather than getting stuck on a stale
	// read from earlier in the same pass.
	return append(containerCreationGateInputs(),
		// The current execution: phase, PID, exit code. Also facilitates dependsOn.containers.
		controller.Input{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
		// Needed to detect an instance stopping: RuntimeController flips ContainerInstanceStatus.Phase
		// to Terminated only once the task has actually exited, so the teardown itself is only visible
		// on the spec's own resource phase.
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

	containerInstanceStatus, err := latestInstanceStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	containerImageStatus, err := safe.ReaderGetByID[*containers.ContainerImageStatus](ctx, runtime, containerSpecID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return optional.None[time.Duration](), fmt.Errorf("failed to get image status %q: %w", containerSpecID, err)
		}

		containerImageStatus = nil
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

	containerInstanceSpec, err := currentInstanceSpec(ctx, runtime, containerSpecID, containerInstanceStatus)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	// The digest which the current reference resolves to, which is not necessarily the one actually running.
	imageDigest, err := containers.GetImageDigest(ctx, runtime, containerSpecID, containerSpec.TypedSpec().Image.Ref)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to resolve image digest %q: %w", containerSpecID, err)
	}

	var before, after containers.ContainerStatusSpec

	if err := safe.WriterModify(ctx, runtime,
		containers.NewContainerStatus(containers.NamespaceName, containerSpecID),
		func(res *containers.ContainerStatus) error {
			before = *res.TypedSpec()

			assembleStatus(res.TypedSpec(), containerSpec, containerImageStatus, containerInstanceStatus, containerInstanceSpec, imageDigest, waitingFor, before)

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
func latestInstanceStatus(
	ctx context.Context,
	reader controller.Reader,
	containerSpecID string,
) (*containers.ContainerInstanceStatus, error) {
	containerInstanceStatuses, err := safe.ReaderListAll[*containers.ContainerInstanceStatus](ctx, reader,
		state.WithIDQuery(containers.InstanceIDQuery(containerSpecID)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance statuses %q: %w", containerSpecID, err)
	}

	var latestStatus *containers.ContainerInstanceStatus

	for containerInstanceStatus := range containerInstanceStatuses.All() {
		if latestStatus == nil || containerInstanceStatus.TypedSpec().Generation > latestStatus.TypedSpec().Generation {
			latestStatus = containerInstanceStatus
		}
	}

	return latestStatus, nil
}

// currentInstanceSpec returns the spec of containerSpecID's current instance, or nil if there is none.
func currentInstanceSpec(
	ctx context.Context,
	reader controller.Reader,
	containerSpecID string,
	containerInstanceStatus *containers.ContainerInstanceStatus,
) (*containers.ContainerInstanceSpec, error) {
	if containerInstanceStatus == nil {
		return nil, nil
	}

	instanceSpecID := containers.InstanceID(containerSpecID, containerInstanceStatus.TypedSpec().Generation)

	containerInstanceSpec, err := safe.ReaderGetByID[*containers.ContainerInstanceSpec](ctx, reader, instanceSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get instance spec %q: %w", instanceSpecID, err)
	}

	return containerInstanceSpec, nil
}

// logTransition reports a change in the aggregated state or error of one container.
func logTransition(logger *zap.Logger, containerSpecID string, before, after containers.ContainerStatusSpec) {
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

// assembleStatus derives the aggregated status for one container from its already-resolved inputs.
//
// prevContainerStatusSpec is the status as it was before this pass. Most of it is recomputed from
// scratch, but the few values whose source disappears from under them — Health while an instance is
// on its way out, and the last execution's outcome between instances — carry over from it.
func assembleStatus(
	containerStatusSpec *containers.ContainerStatusSpec,
	containerSpec *containers.ContainerSpec,
	containerImageStatus *containers.ContainerImageStatus,
	containerInstanceStatus *containers.ContainerInstanceStatus,
	containerInstanceSpec *containers.ContainerInstanceSpec,
	imageDigest string,
	waitingFor []string,
	prevContainerStatusSpec containers.ContainerStatusSpec,
) {
	isStopping := containerInstanceSpec != nil && containerInstanceSpec.Metadata().Phase() == resource.PhaseTearingDown

	containerStatusSpec.Image = resolveReportedImage(containerSpec, containerInstanceSpec, imageDigest)

	containerStatusSpec.PID = 0
	containerStatusSpec.WaitingFor = nil

	// There is no instance status between generations, nor for as long as a restart waits on a gate.
	// The last execution's outcome is what an operator is looking at in exactly those windows, so it
	// survives the gap rather than reading back as a container that never crashed.
	containerStatusSpec.RestartCount = prevContainerStatusSpec.RestartCount
	containerStatusSpec.ExitCode = prevContainerStatusSpec.ExitCode

	if containerInstanceStatus != nil {
		containerStatusSpec.RestartCount = containerInstanceStatus.TypedSpec().Generation

		containerStatusSpec.ExitCode = containerInstanceStatus.TypedSpec().ExitCode
		if containerInstanceStatus.TypedSpec().Phase == containers.ContainerInstancePhaseRunning {
			containerStatusSpec.PID = containerInstanceStatus.TypedSpec().PID
		}
	}

	containerStatusSpec.State = resolveContainerState(containerInstanceStatus, containerImageStatus, len(waitingFor) == 0, isStopping)

	if containerStatusSpec.State == containers.ContainerStateStopping {
		containerStatusSpec.Health = prevContainerStatusSpec.Health
	} else {
		containerStatusSpec.Health = containerStatusSpec.State.Health()
	}

	if containerStatusSpec.State == containers.ContainerStatePending {
		containerStatusSpec.WaitingFor = waitingFor
	}

	containerStatusSpec.Error = deriveReportedError(containerInstanceStatus, containerImageStatus)

	// Same reasoning as RestartCount and ExitCode: keep the reason the last execution ended visible
	// while the status that reported it is gone.
	if containerStatusSpec.Error == "" && containerInstanceStatus == nil {
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

// deriveReportedError picks the error to report, preferring the running execution's over the image's.
func deriveReportedError(containerInstanceStatus *containers.ContainerInstanceStatus, containerImageStatus *containers.ContainerImageStatus) string {
	if containerInstanceStatus != nil && containerInstanceStatus.TypedSpec().Error != "" {
		return containerInstanceStatus.TypedSpec().Error
	}

	if containerImageStatus != nil && containerImageStatus.TypedSpec().Error != "" {
		return containerImageStatus.TypedSpec().Error
	}

	return ""
}

// resolveContainerState maps the observable resources onto a container state.
//
// There is no terminal state: a finished instance means a restart is pending, which is exited while
// it is still fresh and backoff once RestartInterval has elapsed waiting for it.
func resolveContainerState(
	containerInstanceStatus *containers.ContainerInstanceStatus,
	containerImageStatus *containers.ContainerImageStatus,
	gatesReady bool,
	isStopping bool,
) containers.ContainerState {
	if containerInstanceStatus != nil {
		if isStopping {
			return containers.ContainerStateStopping
		}

		return resolveInstanceState(containerInstanceStatus)
	}

	// Only the phases that say something the gate check cannot: an image still coming down, or one
	// that will not. Pending and Ready are left to the gate check below, which reports "image" as
	// unmet for as long as there is no usable digest.
	if containerImageStatus != nil {
		switch containerImageStatus.TypedSpec().Phase {
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
func restartWindowWakeAfter(containerInstanceStatus *containers.ContainerInstanceStatus) optional.Optional[time.Duration] {
	if containerInstanceStatus == nil {
		return optional.None[time.Duration]()
	}

	switch containerInstanceStatus.TypedSpec().Phase {
	case containers.ContainerInstancePhaseTerminated, containers.ContainerInstancePhaseFailed:
		if remaining := RestartInterval - time.Since(containerInstanceStatus.TypedSpec().FinishedAt); remaining > 0 {
			return optional.Some(remaining)
		}
	case containers.ContainerInstancePhaseCreated, containers.ContainerInstancePhaseRunning:
	}

	return optional.None[time.Duration]()
}

func resolveInstanceState(containerInstanceStatus *containers.ContainerInstanceStatus) containers.ContainerState {
	switch containerInstanceStatus.TypedSpec().Phase {
	case containers.ContainerInstancePhaseCreated:
		return containers.ContainerStateStarting
	case containers.ContainerInstancePhaseRunning:
		return containers.ContainerStateRunning
	case containers.ContainerInstancePhaseTerminated, containers.ContainerInstancePhaseFailed:
		if time.Since(containerInstanceStatus.TypedSpec().FinishedAt) < RestartInterval {
			return containers.ContainerStateExited
		}

		return containers.ContainerStateBackoff
	}

	return containers.ContainerStateStarting
}
