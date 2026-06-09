// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clock_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/time/internal/clock"
)

func TestWallClockJumpDetectorNoJump(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jumpDetector := clock.NewWallClockJumpDetector(100*time.Millisecond, 200*time.Millisecond)

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		jumpCh := jumpDetector.Run(ctx)

		synctest.Wait()

		// run a cycle without a jump
		time.Sleep(100 * time.Millisecond)

		select {
		case <-jumpCh:
			t.Fatal("no jump expected")
		default:
		}
	})
}

func TestWallClockJumpDetectorJump(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jumpDetector := clock.NewWallClockJumpDetector(100*time.Millisecond, 200*time.Millisecond)

		// Simulate a jump by offsetting the stored wall-clock baseline before the
		// detector goroutine starts. Doing it before Run starts the goroutine means
		// the `go` statement provides the happens-before edge, so there's no data
		// race on the baseline (which the detector goroutine reads and updates).
		jumpDetector.SetWallClock(time.Now().Add(time.Second))

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		jumpCh := jumpDetector.Run(ctx)

		// run a cycle with a jump
		time.Sleep(100 * time.Millisecond)

		select {
		case <-jumpCh:
			// Jump detected as expected.
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected to detect a wall clock jump, but did not")
		}

		// now, as the jump was already detected, it should not be reported anymore
		time.Sleep(100 * time.Millisecond)

		select {
		case <-jumpCh:
			t.Fatal("no jump expected")
		default:
		}
	})
}
