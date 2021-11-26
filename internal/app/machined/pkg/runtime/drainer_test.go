// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

//nolint:gocyclo
func TestDrainer(t *testing.T) {
	drainer := runtime.NewDrainer()

	sub1 := drainer.Subscribe()
	sub2 := drainer.Subscribe()

	errCh := make(chan error)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		errCh <- drainer.Drain(ctx)
	}()

	select {
	case <-sub1.EventCh():
	case <-time.After(time.Second):
		require.Fail(t, "should be notified")
	}

	select {
	case <-sub2.EventCh():
	case <-time.After(time.Second):
		require.Fail(t, "should be notified")
	}

	select {
	case <-errCh:
		require.Fail(t, "shouldn't be drained now")
	default:
	}

	sub1.Cancel()

	select {
	case <-errCh:
		require.Fail(t, "shouldn't be drained now")
	default:
	}

	sub3 := drainer.Subscribe()

	select {
	case <-sub3.EventCh():
	case <-time.After(time.Second):
		require.Fail(t, "should be notified")
	}

	sub3.Cancel()
	sub2.Cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		require.Fail(t, "should be drained now")
	}
}

func TestDrainTimeout(t *testing.T) {
	drainer := runtime.NewDrainer()

	drainer.Subscribe()

	errCh := make(chan error)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		errCh <- drainer.Drain(ctx)
	}()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		require.Fail(t, "should be drained now")
	}
}
