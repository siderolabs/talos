// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Succeeded reports whether a finished instance ran to completion with exit code 0.
//
// A failed start reports exit code 0 as well, which is not a success, and so does an error after the
// start (e.g. a failed wait): the exit code is unknown then.
func (instanceStatusSpec ContainerInstanceStatusSpec) Succeeded() bool {
	return instanceStatusSpec.Phase == ContainerInstancePhaseTerminated && instanceStatusSpec.ExitCode == 0 && instanceStatusSpec.Error == ""
}

// RestartPermitted reports whether a container is restarted after its instance finished.
func (restart ContainerRestartSpec) RestartPermitted(runStatus ContainerRunStatusSpec, instanceStatus ContainerInstanceStatusSpec) bool {
	switch restart.Policy {
	case ContainerRestartPolicyNever:
		return false
	case ContainerRestartPolicyOnFailure:
		if instanceStatus.Succeeded() {
			return false
		}

		return restart.MaxRetries == nil || runStatus.Restarts < *restart.MaxRetries
	case ContainerRestartPolicyAlways:
	}

	// always, and an unknown policy, which validation keeps from getting here
	return true
}

// RestartBackoff returns the backoff step and the delay of the restart after an instance finished.
//
// The delay doubles with every restart up to MaxDelay, and starts over once a run lasted at least
// MaxDelay. A failed start counts as a short run.
func (restart ContainerRestartSpec) RestartBackoff(runStatus ContainerRunStatusSpec, instanceStatus ContainerInstanceStatusSpec) (uint32, time.Duration) {
	delay := cmp.Or(restart.Delay, constants.ContainerDefaultRestartDelay)
	maxDelay := max(restart.MaxDelay, delay)

	step := runStatus.BackoffStep

	if !instanceStatus.StartedAt.IsZero() && instanceStatus.FinishedAt.Sub(instanceStatus.StartedAt) >= maxDelay {
		step = 0
	}

	for range step {
		if delay > maxDelay/2 {
			return step, maxDelay
		}

		delay *= 2
	}

	return step, min(delay, maxDelay)
}

// ContainerSucceeded reports whether a container ran successfully and is not run again: its latest
// instance runs the current configuration and InstanceController decided it succeeded.
func ContainerSucceeded(ctx context.Context, r controller.Reader, containerID string) (bool, error) {
	containerSpec, err := safe.ReaderGetByID[*ContainerSpec](ctx, r, containerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get container spec %q: %w", containerID, err)
	}

	instanceStatus, err := LatestInstanceStatus(ctx, r, containerID)
	if err != nil {
		return false, err
	}

	if instanceStatus == nil {
		return false, nil
	}

	instanceSpec, err := CurrentInstanceSpec(ctx, r, containerID, instanceStatus)
	if err != nil {
		return false, err
	}

	if instanceSpec == nil || instanceSpec.Metadata().Phase() != resource.PhaseRunning {
		return false, nil
	}

	return succeededOutcome(ctx, r, containerSpec, instanceSpec, instanceStatus.TypedSpec().Generation)
}

// succeededOutcome reports whether InstanceController decided the instance of the generation
// succeeded, as a run of the current configuration of the container.
func succeededOutcome(
	ctx context.Context,
	r controller.Reader,
	containerSpec *ContainerSpec,
	instanceSpec *ContainerInstanceSpec,
	generation uint64,
) (bool, error) {
	containerID := containerSpec.Metadata().ID()

	containerRunStatus, err := safe.ReaderGetByID[*ContainerRunStatus](ctx, r, containerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get run status %q: %w", containerID, err)
	}

	runStatus := containerRunStatus.TypedSpec()

	// The run status is checked against the spec as well: InstanceController may not have caught up
	// with a change of it yet, which makes the outcome one of a configuration no longer wanted.
	if runStatus.Replaced(*instanceSpec.TypedSpec()) || runStatus.ExecutionHash != containerSpec.TypedSpec().ExecutionHash() {
		return false, nil
	}

	outcome, ok := runStatus.Outcome(generation)

	return ok && outcome == ContainerStateSucceeded, nil
}

// FinishedState returns the state of a container whose newest instance has finished.
//
// An instance which Talos refused to start is replaced regardless of the restart policy.
func (restart ContainerRestartSpec) FinishedState(runStatus ContainerRunStatusSpec, instanceStatus ContainerInstanceStatusSpec) ContainerState {
	switch {
	case instanceStatus.Refused, runStatus.RestartCharged(instanceStatus.Generation), restart.RestartPermitted(runStatus, instanceStatus):
		return ContainerStateExited
	case instanceStatus.Succeeded():
		return ContainerStateSucceeded
	default:
		return ContainerStateFailed
	}
}
