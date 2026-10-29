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
// A failed start reports exit code 0 as well, which is not a success.
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
		return true
	default:
		return true
	}
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
// instance succeeded with the current configuration, no restart is pending and the restart policy
// doesn't restart it.
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

	if instanceStatus == nil || !instanceStatus.TypedSpec().Succeeded() {
		return false, nil
	}

	instanceSpec, err := CurrentInstanceSpec(ctx, r, containerID, instanceStatus)
	if err != nil {
		return false, err
	}

	if !runsConfiguration(instanceSpec, containerSpec.TypedSpec().ExecutionHash()) {
		return false, nil
	}

	runStatus, err := currentRunStatus(ctx, r, containerID)
	if err != nil {
		return false, err
	}

	return containerSpec.TypedSpec().Restart.FinishedState(runStatus, *instanceStatus.TypedSpec()) == ContainerStateCompleted, nil
}

// runsConfiguration reports whether an instance runs the configuration with the execution hash, and
// isn't being replaced.
func runsConfiguration(instanceSpec *ContainerInstanceSpec, executionHash string) bool {
	return instanceSpec != nil && instanceSpec.Metadata().Phase() == resource.PhaseRunning &&
		instanceSpec.TypedSpec().ExecutionHash == executionHash
}

// currentRunStatus returns the restart accounting of a container, empty if there is none yet.
func currentRunStatus(ctx context.Context, r controller.Reader, containerID string) (ContainerRunStatusSpec, error) {
	containerRunStatus, err := safe.ReaderGetByID[*ContainerRunStatus](ctx, r, containerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return ContainerRunStatusSpec{}, nil
		}

		return ContainerRunStatusSpec{}, fmt.Errorf("failed to get run status %q: %w", containerID, err)
	}

	return *containerRunStatus.TypedSpec(), nil
}

// FinishedState returns the state of a container whose newest instance has finished.
func (restart ContainerRestartSpec) FinishedState(runStatus ContainerRunStatusSpec, instanceStatus ContainerInstanceStatusSpec) ContainerState {
	switch {
	case runStatus.RestartCharged(instanceStatus.Generation), restart.RestartPermitted(runStatus, instanceStatus):
		return ContainerStateExited
	case instanceStatus.Succeeded():
		return ContainerStateCompleted
	default:
		return ContainerStateFailed
	}
}
