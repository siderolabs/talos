// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"cmp"
	"time"

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
