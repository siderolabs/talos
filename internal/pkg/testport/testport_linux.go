// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package testport provides loopback ports for tests.
package testport

import (
	"errors"
	"fmt"
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

// Listenable returns a port which the code under test can listen on for TCP, on any address, and
// which stays reserved until the test ends.
//
// The port is held by a bound, never-listening socket with SO_REUSEADDR: the kernel no longer hands
// it to ephemeral binds or connects, while Go listeners, which set SO_REUSEADDR, can still bind it,
// also after closing and rebinding. The socket is dual-stack.
func Listenable(t *testing.T) int {
	t.Helper()

	fd, port := reserveTCP(t)

	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck

	return port
}

// ListenableTCPUDP is [Listenable] for code which listens on the same port for both TCP and UDP on
// 127.0.0.53 and [::1].
//
// Go UDP sockets have no SO_REUSEADDR, so the UDP side is held on 127.0.0.2 instead, which blocks
// wildcard ephemeral binds only.
//
// shortcut: an ephemeral UDP bind to a specific address overlapping 127.0.0.53 or [::1] can still
// take the port, and the code under test then fails to bind while that socket stays open, up to the
// test timeout; switch to a retry in the test if this flakes.
func ListenableTCPUDP(t *testing.T) int {
	t.Helper()

	for range 10 {
		tcpFD, port := reserveTCP(t)

		udpFD, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			unix.Close(tcpFD) //nolint:errcheck

			require.NoError(t, err)
		}

		err = unix.Bind(udpFD, &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 2}, Port: port})
		if err != nil {
			unix.Close(tcpFD) //nolint:errcheck
			unix.Close(udpFD) //nolint:errcheck

			if errors.Is(err, unix.EADDRINUSE) {
				continue
			}

			require.NoError(t, err)
		}

		t.Cleanup(func() {
			unix.Close(tcpFD) //nolint:errcheck
			unix.Close(udpFD) //nolint:errcheck
		})

		return port
	}

	t.Fatal("failed to reserve a port for both TCP and UDP")

	return 0
}

// reserveTCP binds the reservation socket for [Listenable]; the caller owns the returned fd.
func reserveTCP(t *testing.T) (int, int) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)

	port, err := bindReservation(fd)
	if err != nil {
		unix.Close(fd) //nolint:errcheck

		require.NoError(t, err)
	}

	return fd, port
}

func bindReservation(fd int) (int, error) {
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		return 0, err
	}

	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, 0); err != nil {
		return 0, err
	}

	if err := unix.Bind(fd, &unix.SockaddrInet6{}); err != nil {
		return 0, err
	}

	addr, err := unix.Getsockname(fd)
	if err != nil {
		return 0, err
	}

	inet6, ok := addr.(*unix.SockaddrInet6)
	if !ok {
		return 0, fmt.Errorf("unexpected socket address %T", addr)
	}

	return inet6.Port, nil
}
