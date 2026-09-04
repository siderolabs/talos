// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package stall_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action/internal/stall"
)

func TestDetectorFires(t *testing.T) {
	ctx := t.Context()

	stalled := make(chan struct{})

	d := stall.NewDetector(ctx, 50*time.Millisecond, func() { close(stalled) })

	assert.False(t, d.Stalled())

	select {
	case <-stalled:
	case <-time.After(time.Second):
		t.Fatal("stall detector did not fire")
	}

	assert.True(t, d.Stalled())
}

func TestDetectorPokeDelaysFiring(t *testing.T) {
	ctx := t.Context()

	stalled := make(chan struct{})

	const timeout = 100 * time.Millisecond

	d := stall.NewDetector(ctx, timeout, func() { close(stalled) })

	// keep poking for well over the timeout: the detector must not fire while there is progress
	for range 10 {
		time.Sleep(timeout / 4)

		d.Poke()

		require.False(t, d.Stalled(), "fired while being poked")
	}

	// stop poking, and it fires
	select {
	case <-stalled:
	case <-time.After(time.Second):
		t.Fatal("stall detector did not fire once the pokes stopped")
	}
}

func TestDetectorStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	fired := make(chan struct{})

	stall.NewDetector(ctx, 50*time.Millisecond, func() { close(fired) })

	cancel()

	select {
	case <-fired:
		t.Fatal("stall detector fired after its context was canceled")
	case <-time.After(200 * time.Millisecond):
	}
}
