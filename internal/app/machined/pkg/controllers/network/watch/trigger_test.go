// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package watch_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
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

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	trigger := watch.NewRateLimitedTrigger(ctx, mock, 10, 5)

	start := time.Now()

	for time.Since(start) < time.Second {
		trigger.QueueReconcile()
	}

	assert.InDelta(t, int64(14), mock.Get(), 5)
}
