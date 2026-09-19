// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage/internal/retry"
)

func TestTimerReset(t *testing.T) {
	timer := retry.NewTimer(20*time.Millisecond, 80*time.Millisecond)
	t.Cleanup(timer.Stop)

	timer.Schedule()
	timer.Reset()

	select {
	case <-timer.C():
		t.Fatal("stopped retry timer fired")
	case <-time.After(40 * time.Millisecond):
	}

	started := time.Now()
	timer.Schedule()
	<-timer.C()
	require.Less(t, time.Since(started), 60*time.Millisecond)
}

func TestTimerCapsBackoff(t *testing.T) {
	timer := retry.NewTimer(10*time.Millisecond, 20*time.Millisecond)
	t.Cleanup(timer.Stop)

	for range 4 {
		started := time.Now()
		timer.Schedule()
		<-timer.C()
		timer.Fired()

		require.Less(t, time.Since(started), 50*time.Millisecond)
	}
}

func TestTimerScheduleIsIdempotentWhileArmed(t *testing.T) {
	timer := retry.NewTimer(20*time.Millisecond, 80*time.Millisecond)
	t.Cleanup(timer.Stop)

	started := time.Now()
	timer.Schedule()
	timer.Schedule()
	<-timer.C()
	timer.Fired()

	require.Less(t, time.Since(started), 60*time.Millisecond)
}
