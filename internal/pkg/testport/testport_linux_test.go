// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package testport_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/testport"
)

func TestListenable(t *testing.T) {
	t.Parallel()

	port := testport.Listenable(t)

	// The code under test may restart its listeners, so binding must succeed more than once.
	for range 2 {
		for _, addr := range [][2]string{{"tcp4", "0.0.0.0"}, {"tcp6", "[::]"}} {
			l, err := (&net.ListenConfig{}).Listen(t.Context(), addr[0], addr[1]+":"+strconv.Itoa(port))
			require.NoError(t, err, addr)
			require.NoError(t, l.Close())
		}
	}

	requireTaken(t, unix.SOCK_STREAM, port)
}

func TestListenableTCPUDP(t *testing.T) {
	t.Parallel()

	port := testport.ListenableTCPUDP(t)

	requireTaken(t, unix.SOCK_STREAM, port)
	requireTaken(t, unix.SOCK_DGRAM, port)

	var lc net.ListenConfig

	for _, addr := range [][2]string{{"tcp4", "127.0.0.53"}, {"tcp6", "[::1]"}} {
		l, err := lc.Listen(t.Context(), addr[0], addr[1]+":"+strconv.Itoa(port))
		require.NoError(t, err, addr)

		t.Cleanup(func() { l.Close() }) //nolint:errcheck
	}

	for _, addr := range [][2]string{{"udp4", "127.0.0.53"}, {"udp6", "[::1]"}} {
		c, err := lc.ListenPacket(t.Context(), addr[0], addr[1]+":"+strconv.Itoa(port))
		require.NoError(t, err, addr)

		t.Cleanup(func() { c.Close() }) //nolint:errcheck
	}
}

// requireTaken checks the port is unavailable to a wildcard bind without SO_REUSEADDR, which is the
// conflict check the kernel applies when picking an ephemeral port.
func requireTaken(t *testing.T, typ, port int) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, typ|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)

	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck

	require.ErrorIs(t, unix.Bind(fd, &unix.SockaddrInet4{Port: port}), unix.EADDRINUSE,
		"a released port can be handed to another test before the code under test binds it")
}
