// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package trigger_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
)

type mockTrigger struct {
	count atomic.Int64
}

func (t *mockTrigger) QueueReconcile() {
	t.count.Add(1)
}

func (t *mockTrigger) Get() int64 {
	return t.count.Load()
}

func TestRateLimitedTrigger(t *testing.T) {
	mock := &mockTrigger{}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	trig := trigger.NewRateLimitedTrigger(ctx, mock, 10, 5)

	start := time.Now()

	for time.Since(start) < time.Second {
		trig.QueueReconcile()
	}

	assert.InDelta(t, int64(14), mock.Get(), 5)
}

func TestRateLimitedTriggerQueuesTrailingEvent(t *testing.T) {
	mock := &mockTrigger{}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	trig := trigger.NewRateLimitedTrigger(ctx, mock, rate.Every(250*time.Millisecond), 1)

	// Consume the initial burst token.
	trig.QueueReconcile()
	require.Eventually(t, func() bool {
		return mock.Get() == 1
	}, time.Second, time.Millisecond)

	baseline := mock.Get()

	// The worker dequeues this event and blocks in limiter.Wait.
	trig.QueueReconcile()
	require.Never(t, func() bool {
		return mock.Get() > baseline
	}, 50*time.Millisecond, time.Millisecond)

	// This event must remain pending until the rate-limited event is forwarded.
	trig.QueueReconcile()

	require.Eventually(t, func() bool {
		return mock.Get() == baseline+2
	}, time.Second, time.Millisecond)
}
