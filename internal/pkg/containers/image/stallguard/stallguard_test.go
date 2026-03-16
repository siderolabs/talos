// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package stallguard_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/containers/image/stallguard"
)

func TestConnReadStalls(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()

	t.Cleanup(func() {
		assert.NoError(t, client.Close())
		assert.NoError(t, server.Close())
	})

	conn := stallguard.NewConn(client, 50*time.Millisecond)

	// nothing is ever written to the server side, so the read should trip the
	// idle deadline rather than blocking forever.
	_, err := conn.Read(make([]byte, 16))

	var netErr net.Error

	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout(), "expected a timeout error, got %v", err)
}

func TestConnReadProgressResetsDeadline(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()

	t.Cleanup(func() {
		assert.NoError(t, client.Close())
		assert.NoError(t, server.Close())
	})

	const (
		idleTimeout = 100 * time.Millisecond
		interval    = idleTimeout / 5
		writes      = 10
	)

	conn := stallguard.NewConn(client, idleTimeout)

	errCh := make(chan error, 1)

	// trickle bytes at an interval well below idleTimeout: each successful read
	// resets the deadline, so the total transfer (interval*writes) outlasts a
	// single idle window without ever tripping it.
	go func() {
		for range writes {
			time.Sleep(interval)

			if _, err := server.Write([]byte{0}); err != nil {
				errCh <- err

				return
			}
		}

		errCh <- nil
	}()

	buf := make([]byte, 1)

	for range writes {
		n, err := conn.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 1, n)
	}

	require.NoError(t, <-errCh)
}

func TestConnHonorsShorterExternalDeadline(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()

	t.Cleanup(func() {
		assert.NoError(t, client.Close())
		assert.NoError(t, server.Close())
	})

	// idle timeout is large; the caller sets a much shorter read deadline, which
	// must win so the guard does not relax a stricter timeout the caller asked
	// for.
	conn := stallguard.NewConn(client, time.Hour)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(50*time.Millisecond)))

	start := time.Now()

	_, err := conn.Read(make([]byte, 16))

	var netErr net.Error

	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout(), "expected a timeout error, got %v", err)
	assert.Less(t, time.Since(start), time.Minute, "read should honor the shorter caller deadline, not the idle timeout")
}
