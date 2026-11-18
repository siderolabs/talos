// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package testport provides loopback ports for tests.
package testport

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// Closed returns a loopback port nothing listens on, which stays so until the test ends.
func Closed(t *testing.T) uint32 {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)

	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck

	require.NoError(t, unix.Bind(fd, &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}))

	addr, err := unix.Getsockname(fd)
	require.NoError(t, err)

	inet4, ok := addr.(*unix.SockaddrInet4)
	require.True(t, ok, "unexpected socket address %T", addr)

	port, err := strconv.ParseUint(strconv.Itoa(inet4.Port), 10, 16)
	require.NoError(t, err)

	return uint32(port)
}

// ClosedAddress returns a loopback address, as host:port, which nothing listens on until the test ends.
func ClosedAddress(t *testing.T) string {
	t.Helper()

	return net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(Closed(t)), 10))
}

// UnansweredUDPAddress returns a loopback UDP address, as host:port, which nothing answers on until
// the test ends.
func UnansweredUDPAddress(t *testing.T) string {
	t.Helper()

	conn, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { conn.Close() }) //nolint:errcheck

	return conn.LocalAddr().String()
}
