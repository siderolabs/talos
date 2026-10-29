// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestRestartPermitted(t *testing.T) {
	t.Parallel()

	succeeded := containers.ContainerInstanceStatusSpec{Phase: containers.ContainerInstancePhaseTerminated}
	exitedNonZero := containers.ContainerInstanceStatusSpec{Phase: containers.ContainerInstancePhaseTerminated, ExitCode: 3}
	failedStart := containers.ContainerInstanceStatusSpec{Phase: containers.ContainerInstancePhaseFailed, Error: "no such file"}
	runtimeError := containers.ContainerInstanceStatusSpec{Phase: containers.ContainerInstancePhaseTerminated, Error: "task wait failed"}

	for _, test := range []struct {
		name       string
		restart    containers.ContainerRestartSpec
		restarts   uint32
		instance   containers.ContainerInstanceStatusSpec
		permitted  bool
		finished   containers.ContainerState
		succeeded  bool
		charged    bool
		chargedGen uint64
	}{
		{name: "always after success", restart: containers.ContainerRestartSpec{}, instance: succeeded, permitted: true, finished: containers.ContainerStateExited, succeeded: true},
		{name: "always after failure", restart: containers.ContainerRestartSpec{}, instance: exitedNonZero, permitted: true, finished: containers.ContainerStateExited},
		{name: "never after success", restart: never(), instance: succeeded, finished: containers.ContainerStateCompleted, succeeded: true},
		{name: "never after failure", restart: never(), instance: exitedNonZero, finished: containers.ContainerStateFailed},
		{name: "on-failure after success", restart: onFailure(nil), instance: succeeded, finished: containers.ContainerStateCompleted, succeeded: true},
		{name: "on-failure after failure", restart: onFailure(nil), restarts: 100, instance: exitedNonZero, permitted: true, finished: containers.ContainerStateExited},
		{name: "on-failure after a failed start", restart: onFailure(nil), instance: failedStart, permitted: true, finished: containers.ContainerStateExited},
		{name: "on-failure after a runtime error", restart: onFailure(nil), instance: runtimeError, permitted: true, finished: containers.ContainerStateExited},
		{name: "on-failure with retries left", restart: onFailure(new(uint32(2))), restarts: 1, instance: exitedNonZero, permitted: true, finished: containers.ContainerStateExited},
		{name: "on-failure out of retries", restart: onFailure(new(uint32(2))), restarts: 2, instance: exitedNonZero, finished: containers.ContainerStateFailed},
		{name: "on-failure without retries", restart: onFailure(new(uint32(0))), instance: failedStart, finished: containers.ContainerStateFailed},
		{name: "a charged restart is pending", restart: never(), instance: exitedNonZero, finished: containers.ContainerStateExited, charged: true},
		{name: "a restart charged for another generation", restart: never(), instance: exitedNonZero, finished: containers.ContainerStateFailed, charged: true, chargedGen: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			runStatus := containers.ContainerRunStatusSpec{
				Restarts:            test.restarts,
				Restarted:           test.charged,
				RestartedGeneration: test.chargedGen,
			}

			assert.Equal(t, test.succeeded, test.instance.Succeeded())
			assert.Equal(t, test.permitted, test.restart.RestartPermitted(runStatus, test.instance))
			assert.Equal(t, test.finished, test.restart.FinishedState(runStatus, test.instance))
		})
	}
}

func TestRestartBackoff(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

	ranFor := func(d time.Duration) containers.ContainerInstanceStatusSpec {
		return containers.ContainerInstanceStatusSpec{
			Phase:      containers.ContainerInstancePhaseTerminated,
			StartedAt:  finishedAt.Add(-d),
			FinishedAt: finishedAt,
		}
	}

	backoff := containers.ContainerRestartSpec{Delay: time.Second, MaxDelay: 10 * time.Second}

	for _, test := range []struct {
		name     string
		restart  containers.ContainerRestartSpec
		step     uint32
		instance containers.ContainerInstanceStatusSpec
		wantStep uint32
		want     time.Duration
	}{
		{name: "default", restart: containers.ContainerRestartSpec{}, instance: ranFor(time.Second), want: 5 * time.Second},
		{name: "no backoff by default", restart: containers.ContainerRestartSpec{Delay: time.Second}, step: 7, instance: ranFor(0), wantStep: 7, want: time.Second},
		{name: "first restart", restart: backoff, instance: ranFor(0), want: time.Second},
		{name: "doubles", restart: backoff, step: 3, instance: ranFor(0), wantStep: 3, want: 8 * time.Second},
		{name: "capped", restart: backoff, step: 4, instance: ranFor(0), wantStep: 4, want: 10 * time.Second},
		{name: "saturates", restart: backoff, step: 1 << 20, instance: ranFor(0), wantStep: 1 << 20, want: 10 * time.Second},
		{name: "a long run starts over", restart: backoff, step: 4, instance: ranFor(10 * time.Second), want: time.Second},
		{name: "a short run doesn't", restart: backoff, step: 2, instance: ranFor(9 * time.Second), wantStep: 2, want: 4 * time.Second},
		{
			name:     "a failed start is a short run",
			restart:  backoff,
			step:     2,
			instance: containers.ContainerInstanceStatusSpec{Phase: containers.ContainerInstancePhaseFailed, FinishedAt: finishedAt},
			wantStep: 2,
			want:     4 * time.Second,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			step, delay := test.restart.RestartBackoff(containers.ContainerRunStatusSpec{BackoffStep: test.step}, test.instance)

			assert.Equal(t, test.wantStep, step)
			assert.Equal(t, test.want, delay)
		})
	}
}

func never() containers.ContainerRestartSpec {
	return containers.ContainerRestartSpec{Policy: containers.ContainerRestartPolicyNever}
}

func onFailure(maxRetries *uint32) containers.ContainerRestartSpec {
	return containers.ContainerRestartSpec{Policy: containers.ContainerRestartPolicyOnFailure, MaxRetries: maxRetries}
}
