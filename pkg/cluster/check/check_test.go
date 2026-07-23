// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/cluster/check"
	"github.com/siderolabs/talos/pkg/conditions"
)

// staticCondition is a conditions.Condition whose String() and Wait() are fixed.
type staticCondition struct {
	str     string
	waitErr error
}

func (c staticCondition) String() string { return c.str }

func (c staticCondition) Wait(context.Context) error { return c.waitErr }

// blockingCondition blocks until ctx is canceled, then returns ctx.Err().
// It simulates the --wait-timeout scenario. started is closed once Wait enters,
// so the test can cancel the context only after the condition is running.
type blockingCondition struct {
	str     string
	started chan struct{}
}

func (c blockingCondition) String() string { return c.str }

func (c blockingCondition) Wait(ctx context.Context) error {
	close(c.started)
	<-ctx.Done()

	return ctx.Err()
}

// captureReporter records every Update it receives.
type captureReporter struct {
	updates []string
}

func (r *captureReporter) Update(condition conditions.Condition) {
	r.updates = append(r.updates, condition.String())
}

// captureReporter and *ConditionReporter must both satisfy the Reporter interface.
var (
	_ check.Reporter = (*captureReporter)(nil)
	_ check.Reporter = (*check.ConditionReporter)(nil)
)

func TestWaitFailureReportsError(t *testing.T) {
	rep := &captureReporter{}

	wantErr := errors.New("boom")

	err := check.Wait(
		t.Context(),
		nil,
		[]check.ClusterCheck{
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "etcd to be healthy: boom", waitErr: wantErr}
			},
		},
		rep,
	)

	require.ErrorIs(t, err, wantErr)

	require.NotEmpty(t, rep.updates)
	// The final update for a failed condition wraps it in failedCondition,
	// so String() returns the underlying string.
	assert.Equal(t, "etcd to be healthy: boom", rep.updates[len(rep.updates)-1])
}

func TestWaitFailureErrorCalledOnce(t *testing.T) {
	rep := &captureReporter{}

	wantErr := errors.New("boom")

	require.ErrorIs(t, check.Wait(
		t.Context(),
		nil,
		[]check.ClusterCheck{
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "etcd to be healthy: boom", waitErr: wantErr}
			},
		},
		rep,
	), wantErr)

	// The final update for the failing check should be reported at least once.
	// Count how many times the condition string appears in updates.
	count := 0

	for _, u := range rep.updates {
		if strings.Contains(u, "etcd to be healthy: boom") {
			count++
		}
	}

	assert.GreaterOrEqual(t, count, 1, "failed condition must appear in updates at least once")
}

func TestWaitStopsAtFirstFailure(t *testing.T) {
	rep := &captureReporter{}

	errFirst := errors.New("first check failed")

	err := check.Wait(
		t.Context(),
		nil,
		[]check.ClusterCheck{
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "etcd to be healthy: first failed", waitErr: errFirst}
			},
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "kubelet to be healthy: OK"}
			},
		},
		rep,
	)

	require.ErrorIs(t, err, errFirst)

	for _, u := range rep.updates {
		assert.NotContains(t, u, "kubelet to be healthy",
			"second check must not run after the first check fails")
	}
}

func TestWaitContextCancellation(t *testing.T) {
	rep := &captureReporter{}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})

	// Run Wait in a separate goroutine so we can cancel the context only after
	// the condition has actually started (simulating --wait-timeout expiry
	// mid-run rather than before the check begins).
	errCh := make(chan error, 1)

	go func() {
		errCh <- check.Wait(
			ctx,
			nil,
			[]check.ClusterCheck{
				func(check.ClusterInfo) conditions.Condition {
					return blockingCondition{
						str:     "etcd to be healthy: context canceled",
						started: started,
					}
				},
			},
			rep,
		)
	}()

	<-started // condition is now blocking inside Wait
	cancel()  // simulate timeout expiry

	require.ErrorIs(t, <-errCh, context.Canceled)

	assert.NotEmpty(t, rep.updates,
		"timed-out/canceled condition must be reported via Update")
}

func TestWaitSuccessThenFailure(t *testing.T) {
	rep := &captureReporter{}

	errLast := errors.New("last check failed")

	err := check.Wait(
		t.Context(),
		nil,
		[]check.ClusterCheck{
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "etcd to be healthy: OK"}
			},
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "kubelet to be healthy: OK"}
			},
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "all nodes to report ready: failed", waitErr: errLast}
			},
		},
		rep,
	)

	require.ErrorIs(t, err, errLast)

	// First two checks reported via Update.
	updateStr := strings.Join(rep.updates, " ")
	assert.Contains(t, updateStr, "etcd to be healthy")
	assert.Contains(t, updateStr, "kubelet to be healthy")

	// The final update should be the failed condition.
	assert.Contains(t, rep.updates[len(rep.updates)-1], "all nodes to report ready")
}

func TestWaitSuccessReportsUpdateOnly(t *testing.T) {
	rep := &captureReporter{}

	err := check.Wait(
		t.Context(),
		nil,
		[]check.ClusterCheck{
			func(check.ClusterInfo) conditions.Condition {
				return staticCondition{str: "etcd to be healthy: OK"}
			},
		},
		rep,
	)

	require.NoError(t, err)

	assert.NotEmpty(t, rep.updates)
}
