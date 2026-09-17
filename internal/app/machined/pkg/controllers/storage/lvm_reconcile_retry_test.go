// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLVMReconcileRetryReset(t *testing.T) {
	retry := newLVMReconcileRetry(time.Millisecond, 4*time.Millisecond)
	defer retry.stop()

	retry.schedule()
	require.Equal(t, 2*time.Millisecond, retry.next)

	retry.reset()
	require.Equal(t, time.Millisecond, retry.next)
	require.False(t, retry.armed)

	select {
	case <-retry.channel():
		t.Fatal("stopped retry timer fired")
	case <-time.After(5 * time.Millisecond):
	}
}

func TestLVMReconcileRetryCapsBackoff(t *testing.T) {
	retry := newLVMReconcileRetry(time.Millisecond, 4*time.Millisecond)
	defer retry.stop()

	for range 5 {
		retry.schedule()
		retry.fired()
	}

	require.Equal(t, 4*time.Millisecond, retry.next)
}
