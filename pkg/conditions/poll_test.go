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

	"github.com/talos-systems/talos/pkg/conditions"
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
		}, time.Second, time.Millisecond)

		err := cond.Wait(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "Test condition: OK", cond.String())
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
		}, time.Second, time.Millisecond)

		err := cond.Wait(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "Test condition: SKIP", cond.String())
		assert.Equal(t, 2, calls)
	})

	t.Run("Fatal", func(t *testing.T) {
		t.Parallel()

		var calls int
		cond := conditions.PollingCondition("Test condition", func(ctx context.Context) error {
			calls++

			return errors.New("failed")
		}, time.Second, 750*time.Millisecond)

		err := cond.Wait(context.Background())
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Equal(t, "Test condition: failed", cond.String())
		assert.Equal(t, 2, calls)
	})
}
