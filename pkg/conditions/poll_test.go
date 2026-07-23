// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/conditions"
)

func TestPollingCondition(t *testing.T) {
	t.Parallel()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		var calls int

		cond := conditions.PollingCondition("Test condition", func(ctx context.Context) error {
			calls++

			if calls < 2 {
				return errors.New("failed")
			}

			return nil
		}, time.Millisecond)

		err := cond.Wait(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, "Test condition", cond.String())
		assert.Equal(t, 2, calls)
	})

	t.Run("Skip", func(t *testing.T) {
		t.Parallel()

		var calls int

		cond := conditions.PollingCondition("Test condition", func(ctx context.Context) error {
			calls++

			if calls < 2 {
				return errors.New("failed")
			}

			return conditions.ErrSkipAssertion
		}, time.Millisecond)

		err := cond.Wait(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, "Test condition", cond.String())
		assert.Equal(t, 2, calls)
	})

	t.Run("Fatal", func(t *testing.T) {
		t.Parallel()

		var calls int

		cond := conditions.PollingCondition("Test condition", func(ctx context.Context) error {
			calls++

			return errors.New("failed")
		}, 750*time.Millisecond)

		ctx, cancel := context.WithTimeout(t.Context(), 1400*time.Millisecond)
		defer cancel()

		err := cond.Wait(ctx)
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Equal(t, "Test condition", cond.String())
		assert.Equal(t, 2, calls)
	})
}

func TestPollingConditionState(t *testing.T) {
	t.Parallel()

	t.Run("unpolled", func(t *testing.T) {
		t.Parallel()

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return nil
		}, time.Millisecond)

		state, err := cond.(conditions.Stateful).State()
		assert.Equal(t, conditions.StateRunning, state)
		assert.NoError(t, err)
	})

	t.Run("succeeded", func(t *testing.T) {
		t.Parallel()

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return nil
		}, time.Millisecond)

		err := cond.Wait(t.Context())
		assert.NoError(t, err)

		state, lastErr := cond.(conditions.Stateful).State()
		assert.Equal(t, conditions.StateSucceeded, state)
		assert.NoError(t, lastErr)
	})

	t.Run("skipped", func(t *testing.T) {
		t.Parallel()

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return conditions.ErrSkipAssertion
		}, time.Millisecond)

		err := cond.Wait(t.Context())
		assert.NoError(t, err)

		state, lastErr := cond.(conditions.Stateful).State()
		assert.Equal(t, conditions.StateSkipped, state)
		assert.NoError(t, lastErr)
	})

	t.Run("failed", func(t *testing.T) {
		t.Parallel()

		pollErr := errors.New("connection refused")

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return pollErr
		}, time.Millisecond)

		ctx, cancel := context.WithCancel(t.Context())
		cancel() // cancel immediately so Wait exits after the first assertion attempt

		err := cond.Wait(ctx)
		assert.ErrorIs(t, err, context.Canceled)

		state, lastErr := cond.(conditions.Stateful).State()
		assert.Equal(t, conditions.StateFailed, state)
		assert.Equal(t, pollErr, lastErr)
	})
}

func TestDescribe(t *testing.T) {
	t.Parallel()

	t.Run("unpolled", func(t *testing.T) {
		t.Parallel()

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return nil
		}, time.Millisecond)

		assert.Equal(t, "Test: ...", conditions.StatusLine(cond))
	})

	t.Run("succeeded", func(t *testing.T) {
		t.Parallel()

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return nil
		}, time.Millisecond)

		err := cond.Wait(t.Context())
		assert.NoError(t, err)

		assert.Equal(t, "Test: OK", conditions.StatusLine(cond))
	})

	t.Run("skipped", func(t *testing.T) {
		t.Parallel()

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return conditions.ErrSkipAssertion
		}, time.Millisecond)

		err := cond.Wait(t.Context())
		assert.NoError(t, err)

		assert.Equal(t, "Test: SKIP", conditions.StatusLine(cond))
	})

	t.Run("failed", func(t *testing.T) {
		t.Parallel()

		pollErr := errors.New("connection refused")

		cond := conditions.PollingCondition("Test", func(ctx context.Context) error {
			return pollErr
		}, time.Millisecond)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := cond.Wait(ctx)
		assert.ErrorIs(t, err, context.Canceled)

		assert.Equal(t, "Test: connection refused", conditions.StatusLine(cond))
	})

	t.Run("non-Stateful", func(t *testing.T) {
		t.Parallel()

		// A non-Stateful condition just returns its String() unchanged.
		cond := conditions.WaitForFileToExist("test.txt")

		assert.Equal(t, cond.String(), conditions.StatusLine(cond))
	})
}
