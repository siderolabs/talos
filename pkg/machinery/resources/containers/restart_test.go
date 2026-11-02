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

func TestRunStatusFor(t *testing.T) {
	t.Parallel()

	spec := containers.ContainerSpecSpec{
		Image:   containers.ContainerImageSpec{Ref: "ghcr.io/example/migrate:v1.0.0"},
		Restart: containers.ContainerRestartSpec{Policy: containers.ContainerRestartPolicyOnFailure},
	}

	counted := containers.ContainerRunStatusSpec{
		ExecutionHash:       spec.ExecutionHash(),
		Policy:              containers.ContainerRestartPolicyOnFailure,
		NextGeneration:      5,
		Restarts:            3,
		BackoffStep:         2,
		Restarted:           true,
		RestartedGeneration: 4,
		FinishedGeneration:  4,
		FinishedState:       containers.ContainerStateExited,
	}

	changedPolicy := counted
	changedPolicy.Policy = containers.ContainerRestartPolicyAlways

	changedHash := counted
	changedHash.ExecutionHash = "previous"

	for _, test := range []struct {
		name      string
		runStatus containers.ContainerRunStatusSpec
		want      containers.ContainerRunStatusSpec
	}{
		{name: "same spec", runStatus: counted, want: counted},
		{
			name:      "changed policy",
			runStatus: changedPolicy,
			want: containers.ContainerRunStatusSpec{
				ExecutionHash:       counted.ExecutionHash,
				Policy:              containers.ContainerRestartPolicyOnFailure,
				NextGeneration:      5,
				BackoffStep:         2,
				Restarted:           true,
				RestartedGeneration: 4,
				FinishedGeneration:  4,
				FinishedState:       containers.ContainerStateExited,
			},
		},
		{
			name:      "changed configuration",
			runStatus: changedHash,
			want: containers.ContainerRunStatusSpec{
				ExecutionHash:  counted.ExecutionHash,
				Policy:         containers.ContainerRestartPolicyOnFailure,
				NextGeneration: 5,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.runStatus.For(spec.ExecutionHash(), spec.Restart.Policy))
		})
	}
}

func TestFinishedStateRefused(t *testing.T) {
	t.Parallel()

	refused := containers.ContainerInstanceStatusSpec{Phase: containers.ContainerInstancePhaseFailed, Error: "mounts or devices changed", Refused: true}

	for _, restart := range []containers.ContainerRestartSpec{
		{Policy: containers.ContainerRestartPolicyNever},
		{Policy: containers.ContainerRestartPolicyOnFailure, MaxRetries: new(uint32(0))},
	} {
		assert.Equal(t, containers.ContainerStateExited, restart.FinishedState(containers.ContainerRunStatusSpec{}, refused))
	}
}

func TestRunStatusOutcome(t *testing.T) {
	t.Parallel()

	decided := containers.ContainerRunStatusSpec{FinishedGeneration: 3, FinishedState: containers.ContainerStateCompleted}

	outcome, ok := decided.Outcome(3)
	assert.True(t, ok)
	assert.Equal(t, containers.ContainerStateCompleted, outcome)

	_, ok = decided.Outcome(4)
	assert.False(t, ok, "another generation")

	_, ok = containers.ContainerRunStatusSpec{}.Outcome(0)
	assert.False(t, ok, "not decided")
}
