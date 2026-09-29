// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bootpartition_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/bootpartition"
)

const bootPartitionUUID = "6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001"

func TestUnknownBootPartition(t *testing.T) {
	t.Parallel()

	w := bootpartition.NewWaiter(time.Second)
	defer w.Stop()

	// nothing to wait for
	assert.True(t, w.Discovered(zaptest.NewLogger(t), "", slices.Values([]string{"other"})))
	assert.Nil(t, w.TimerC())
}

func TestDiscoveredImmediately(t *testing.T) {
	t.Parallel()

	w := bootpartition.NewWaiter(time.Second)
	defer w.Stop()

	// case-insensitive match
	assert.True(t, w.Discovered(zaptest.NewLogger(t), bootPartitionUUID, slices.Values([]string{"", "6F5A6E8A-9C79-4C0F-8F35-0C2A1F1A0001"})))
	assert.Nil(t, w.TimerC())
}

func TestDiscoveredLater(t *testing.T) {
	t.Parallel()

	w := bootpartition.NewWaiter(time.Minute)
	defer w.Stop()

	logger := zaptest.NewLogger(t)

	// not discovered yet: the wait starts
	assert.False(t, w.Discovered(logger, bootPartitionUUID, slices.Values([]string{"other"})))
	require.NotNil(t, w.TimerC())

	assert.False(t, w.Discovered(logger, bootPartitionUUID, slices.Values([]string{"other"})))

	// discovered: the wait is over
	assert.True(t, w.Discovered(logger, bootPartitionUUID, slices.Values([]string{"other", bootPartitionUUID})))

	// and it never restarts
	assert.True(t, w.Discovered(logger, bootPartitionUUID, slices.Values([]string{"other"})))
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	w := bootpartition.NewWaiter(100 * time.Millisecond)
	defer w.Stop()

	logger := zaptest.NewLogger(t)

	assert.False(t, w.Discovered(logger, bootPartitionUUID, slices.Values([]string{})))
	require.NotNil(t, w.TimerC())

	// the timer wakes up the caller when the wait times out
	select {
	case <-w.TimerC():
	case <-time.After(5 * time.Second):
		t.Fatal("timer didn't fire")
	}

	assert.True(t, w.Discovered(logger, bootPartitionUUID, slices.Values([]string{})))

	// once timed out, the wait is done for good
	assert.True(t, w.Discovered(logger, bootPartitionUUID, slices.Values([]string{})))
}

func TestDefaultTimeout(t *testing.T) {
	t.Parallel()

	w := bootpartition.NewWaiter(0)
	defer w.Stop()

	// the default timeout is long, so the wait is still on
	assert.False(t, w.Discovered(zaptest.NewLogger(t), bootPartitionUUID, slices.Values([]string{})))
	assert.False(t, w.Discovered(zaptest.NewLogger(t), bootPartitionUUID, slices.Values([]string{})))
}
