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
//
// Pure projection: it owns nothing and decides nothing. Its job is to give an operator one resource
// to look at, and to keep the coarse health value stable even as the internal states change.
type StatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *StatusController) Name() string {
	return "containers.StatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StatusController) Inputs() []controller.Input {
	// The ContainerStatus entry gateInputs contributes is self-referential here: dependsOn.containers
	// gates on another container's Health, computed by this same controller. Reads of an own Output
	// are permitted without an Input declaration, but the declaration is what makes it reactive —
	// one container's status write is what schedules the pass that lets a dependent container
	// notice, converging to a fixed point rather than getting stuck on a stale read from earlier in
	// the same pass.
	return append(containerCreationGateInputs(),
		// The current execution: phase, PID, exit code.
		controller.Input{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
		// Needed to detect an instance stopping: RuntimeController flips ContainerInstanceStatus.Phase
		// to Terminated only once the task has actually exited, so the teardown itself — SIGTERM sent,
		// mounts being released — is only visible on the spec's own resource phase.
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
func (ctrl *StatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return runWithWakeTimer(ctx, r, func(ctx context.Context, r controller.Runtime) (optional.Optional[time.Duration], error) {
		wakeAfter, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			logger.Error("failed to aggregate container statuses", zap.Error(err))
		}

		return wakeAfter, err
	})
}

// reconcile returns how long until the controller next needs to wake up on its own, if at all.
func (ctrl *StatusController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (optional.Optional[time.Duration], error) {
	r.StartTrackingOutputs()

	specs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, r)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to list container specs: %w", err)
	}

	instanceStatuses, err := safe.ReaderListAll[*containers.ContainerInstanceStatus](ctx, r)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to list instance statuses: %w", err)
	}

	// Keep only the newest instance status per container: ContainerStatus reflects the current
	// execution, while the per-execution history stays on ContainerInstanceStatus.
	newest := map[string]*containers.ContainerInstanceStatus{}

	for status := range instanceStatuses.All() {
		containerID := status.TypedSpec().ContainerID

		if existing, ok := newest[containerID]; !ok || status.TypedSpec().Generation > existing.TypedSpec().Generation {
			newest[containerID] = status
		}
	}

	var wakeCtrlAfter optional.Optional[time.Duration]

	for spec := range specs.All() {
		containerID := spec.Metadata().ID()

		wakeAfter, err := ctrl.reconcileContainer(ctx, r, logger, containerID, spec, newest[containerID])
		if err != nil {
			return optional.None[time.Duration](), err
		}

		wakeCtrlAfter = minOptionalDuration(wakeCtrlAfter, wakeAfter)
	}

	if err := safe.CleanupOutputs[*containers.ContainerStatus](ctx, r); err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return wakeCtrlAfter, nil
}

// reconcileContainer derives and writes the aggregated status for one container.
func (ctrl *StatusController) reconcileContainer(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	containerID string,
	spec *containers.ContainerSpec,
	instance *containers.ContainerInstanceStatus,
) (optional.Optional[time.Duration], error) {
	imageStatus, err := safe.ReaderGetByID[*containers.ContainerImageStatus](ctx, r, containerID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return optional.None[time.Duration](), fmt.Errorf("failed to get image status %q: %w", containerID, err)
		}

		imageStatus = nil
	}

	// Gates only feed the status while there is no instance: once one exists its own phase is the
	// state, and rechecking them would keep this controller polling dependsOn.paths at 1 Hz for the
	// life of the container.
	var (
		waitingFor []string
		wakeAfter  optional.Optional[time.Duration]
	)

	if instance == nil {
		waitingFor, wakeAfter, err = spec.TypedSpec().Ready(ctx, r, containerID)
		if err != nil {
			return optional.None[time.Duration](), fmt.Errorf("failed to check container ready %q: %w", containerID, err)
		}

		if len(waitingFor) == 0 {
			// Nothing is gated, so a path recheck has nothing left to notice. InstanceController
			// creating the instance is the event that brings us back.
			wakeAfter = optional.None[time.Duration]()
		}
	}

	instanceSpec, err := ctrl.currentInstanceSpec(ctx, r, containerID, instance)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	// The digest the current reference resolves to, which is not necessarily the one running: see
	// project. GetImageDigest is what InstanceController resolves an instance's image with, and it
	// carries the rule that a status describing some other reference does not count.
	digest, err := containers.GetImageDigest(ctx, r, containerID, spec.TypedSpec().Image.Ref)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to resolve image digest %q: %w", containerID, err)
	}

	var before, after containers.ContainerStatusSpec

	if err := safe.WriterModify(ctx, r,
		containers.NewContainerStatus(containers.NamespaceName, containerID),
		func(res *containers.ContainerStatus) error {
			before = *res.TypedSpec()

			project(res.TypedSpec(), spec, imageStatus, instance, instanceSpec, digest, waitingFor, before)

			after = *res.TypedSpec()

			return nil
		},
	); err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to write container status %q: %w", containerID, err)
	}

	// This is the one place with a before-and-after view of the aggregate, so it is where a state
	// change is worth a line in the log rather than in every controller that causes one.
	logTransition(logger, containerID, before, after)

	return minOptionalDuration(wakeAfter, restartWindowWakeAfter(instance)), nil
}

// currentInstanceSpec returns the spec of containerID's current instance, or nil if there is none.
//
// It is the only view of two things the instance's status does not carry: the image digest actually
// running, and a teardown already in progress. ContainerInstanceStatus.Phase only flips to
// Terminated once the task has actually exited, so a teardown under way — SIGTERM sent, mounts
// being released — is visible on the ContainerInstanceSpec's own resource phase and nowhere else.
func (ctrl *StatusController) currentInstanceSpec(
	ctx context.Context,
	r controller.Reader,
	containerID string,
	instance *containers.ContainerInstanceStatus,
) (*containers.ContainerInstanceSpec, error) {
	if instance == nil {
		return nil, nil
	}

	instanceSpecID := containers.InstanceID(containerID, instance.TypedSpec().Generation)

	instanceSpec, err := safe.ReaderGetByID[*containers.ContainerInstanceSpec](ctx, r, instanceSpecID)
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

// project derives the aggregated status for one container from its already-resolved inputs.
//
// previous is the status as it was before this pass. Most of it is recomputed from scratch, but the
// few values whose source disappears from under them — Health while an instance is on its way out,
// and the last execution's outcome between instances — carry over from it.
func project(
	status *containers.ContainerStatusSpec,
	spec *containers.ContainerSpec,
	imageStatus *containers.ContainerImageStatus,
	instance *containers.ContainerInstanceStatus,
	instanceSpec *containers.ContainerInstanceSpec,
	digest string,
	waitingFor []string,
	previous containers.ContainerStatusSpec,
) {
	stopping := instanceSpec != nil && instanceSpec.Metadata().Phase() == resource.PhaseTearingDown

	status.Image = projectImage(spec, instanceSpec, digest)

	status.PID = 0
	status.WaitingFor = nil

	// There is no instance status between generations, nor for as long as a restart waits on a gate.
	// The last execution's outcome is what an operator is looking at in exactly those windows, so it
	// survives the gap rather than reading back as a container that never crashed.
	status.RestartCount = previous.RestartCount
	status.ExitCode = previous.ExitCode

	if instance != nil {
		status.RestartCount = instance.TypedSpec().Generation

		status.ExitCode = instance.TypedSpec().ExitCode
		if instance.TypedSpec().Phase == containers.ContainerInstancePhaseRunning {
			status.PID = instance.TypedSpec().PID
		}
	}

	status.State = deriveState(instance, imageStatus, len(waitingFor) == 0, stopping)

	if status.State == containers.ContainerStateStopping {
		status.Health = previous.Health
	} else {
		status.Health = status.State.Health()
	}

	if status.State == containers.ContainerStatePending {
		status.WaitingFor = waitingFor
	}

	status.Error = selectError(instance, imageStatus)

	// Same reasoning as RestartCount and ExitCode: keep the reason the last execution ended visible
	// while the status that reported it is gone.
	if status.Error == "" && instance == nil {
		status.Error = previous.Error
	}
}

// projectImage picks the image to report.
func projectImage(spec *containers.ContainerSpec, instanceSpec *containers.ContainerInstanceSpec, digest string) string {
	if instanceSpec != nil && instanceSpec.TypedSpec().Image != "" {
		return instanceSpec.TypedSpec().Image
	}

	if digest != "" {
		return digest
	}

	return spec.TypedSpec().Image.Ref
}

func selectError(instance *containers.ContainerInstanceStatus, imageStatus *containers.ContainerImageStatus) string {
	if instance != nil && instance.TypedSpec().Error != "" {
		return instance.TypedSpec().Error
	}

	if imageStatus != nil && imageStatus.TypedSpec().Error != "" {
		return imageStatus.TypedSpec().Error
	}

	return ""
}

// deriveState maps the observable resources onto a container state.
//
// There is no terminal state: a finished instance means a restart is pending, which is exited while
// it is still fresh and backoff once RestartInterval has elapsed waiting for it.
func deriveState(
	instance *containers.ContainerInstanceStatus,
	imageStatus *containers.ContainerImageStatus,
	gatesReady bool,
	stopping bool,
) containers.ContainerState {
	if instance != nil {
		if stopping {
			return containers.ContainerStateStopping
		}

		return instanceState(instance)
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
