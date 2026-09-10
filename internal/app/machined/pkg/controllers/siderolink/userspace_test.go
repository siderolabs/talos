// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/siderolink"
)

func TestResettableTimer(t *testing.T) {
	t.Parallel()

	t.Run("a retry armed while the loop is parked wakes it", func(t *testing.T) {
		t.Parallel()

		timer := siderolink.NewResettableTimer()

		timer.Reset(0)

		// Capture the channel the way a parked select does, before the retry is armed.
		parked := timer.C()
		require.NotNil(t, parked, "the timer channel must stay non-nil while the timer is stopped")

		timer.Reset(10 * time.Millisecond)

		select {
		case <-parked:
		case <-time.After(time.Second):
			t.Fatal("the retry never fired, the relay would never be restarted")
		}
	})

	t.Run("reset to zero stops a pending firing", func(t *testing.T) {
		t.Parallel()

		timer := siderolink.NewResettableTimer()

		timer.Reset(10 * time.Millisecond)
		timer.Reset(0)

		select {
		case <-timer.C():
			t.Fatal("the timer fired after it was stopped with Reset(0)")
		case <-time.After(100 * time.Millisecond):
		}
	})
}
