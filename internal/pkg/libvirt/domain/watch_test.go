// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

// The fixture uses the real RPC transport; registration is acknowledged only
// after the handshake, so Watch's return establishes a subscription barrier.
func watchFixture(t *testing.T) (net.Conn, net.Conn, <-chan error) {
	t.Helper()

	raw, server := net.Pipe()
	served := make(chan error, 1)

	go func() {
		if err := serveHandshake(server); err != nil {
			served <- err

			return
		}

		call, err := readCall(server)
		if err != nil {
			served <- err

			return
		}

		if got := binary.BigEndian.Uint32(call[8:12]); got != 316 {
			served <- fmt.Errorf("register procedure = %d, want 316", got)

			return
		}

		if got := binary.BigEndian.Uint32(call[24:28]); got != 0 {
			served <- fmt.Errorf("event ID = %d, want lifecycle 0", got)

			return
		}

		served <- replyCall(server, call, binary.BigEndian.AppendUint32(nil, 7))
	}()

	t.Cleanup(func() {
		require.NoError(t, raw.Close())
		require.NoError(t, server.Close())
	})

	return raw, server, served
}

func watchClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case _, ok := <-ch:
		require.False(t, ok, "watch must close, not send a value")
	case <-time.After(time.Second):
		t.Fatal("watch did not close promptly")
	}
}

func TestWatchConnRegistrationAndCancellation(t *testing.T) {
	raw, _, served := watchFixture(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch, err := libvirtdomain.New("", "qemu:///system").WatchConn(ctx, raw)
	require.NoError(t, err)
	require.NoError(t, <-served)
	cancel()
	watchClosed(t, ch)
}

func TestWatchConnLifecycleEventCoalesces(t *testing.T) {
	raw, server, served := watchFixture(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch, err := libvirtdomain.New("", "qemu:///system").WatchConn(ctx, raw)
	require.NoError(t, err)
	require.NoError(t, <-served)

	payload := binary.BigEndian.AppendUint32(nil, 7) // callback ID
	payload = append(payload, encodeDomain(libvirtdomain.Domain{Name: "guest", UUID: uuid.New()})...)
	payload = binary.BigEndian.AppendUint32(payload, 2) // event
	payload = binary.BigEndian.AppendUint32(payload, 0) // detail

	frame := make([]byte, 28, 28+len(payload))
	binary.BigEndian.PutUint32(frame[:4], uint32(28+len(payload)))
	binary.BigEndian.PutUint32(frame[4:8], 0x20008086) // REMOTE_PROGRAM
	binary.BigEndian.PutUint32(frame[8:12], 1)         // version
	binary.BigEndian.PutUint32(frame[12:16], 318)      // DOMAIN_EVENT_CALLBACK_LIFECYCLE
	binary.BigEndian.PutUint32(frame[16:20], 2)        // REMOTE_MESSAGE
	frame = append(frame, payload...)

	_, err = server.Write(frame)
	require.NoError(t, err)

	select {
	case _, ok := <-ch:
		require.True(t, ok, "event must wake the watcher")
	case <-time.After(time.Second):
		t.Fatal("lifecycle event did not wake watcher")
	}

	cancel()
	watchClosed(t, ch)
}

func TestWatchConnConnectionDeathClosesChannel(t *testing.T) {
	raw, server, served := watchFixture(t)
	ch, err := libvirtdomain.New("", "qemu:///system").WatchConn(t.Context(), raw)
	require.NoError(t, err)
	require.NoError(t, <-served)
	require.NoError(t, server.Close())
	watchClosed(t, ch)
}

func TestWatchConnCanceledDuringHandshake(t *testing.T) {
	raw, server := net.Pipe()
	defer server.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { _, err := libvirtdomain.New("", "qemu:///system").WatchConn(ctx, raw); done <- err }()

	call, err := readCall(server)
	require.NoError(t, err)
	require.EqualValues(t, 66, binary.BigEndian.Uint32(call[8:12]))
	cancel()

	select {
	case err = <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		require.NoError(t, raw.Close())

		t.Fatal("handshake did not stop on cancellation")
	}
}

func TestWatchConnCanceledDuringRegistration(t *testing.T) {
	raw, server := net.Pipe()
	defer server.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { _, err := libvirtdomain.New("", "qemu:///system").WatchConn(ctx, raw); done <- err }()

	require.NoError(t, serveHandshake(server))

	call, err := readCall(server)
	require.NoError(t, err)
	require.EqualValues(t, 316, binary.BigEndian.Uint32(call[8:12]))
	cancel()

	select {
	case err = <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		require.NoError(t, raw.Close())

		t.Fatal("registration did not stop on cancellation")
	}
}

func TestWatchUsesDedicatedLongLivedConnection(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "qemu.sock")

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)

	defer listener.Close() //nolint:errcheck

	served := make(chan error, 1)

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			served <- acceptErr

			return
		}

		defer conn.Close() //nolint:errcheck

		if err := serveHandshake(conn); err != nil {
			served <- err

			return
		}

		call, err := readCall(conn)
		if err != nil {
			served <- err

			return
		}

		if got := binary.BigEndian.Uint32(call[8:12]); got != 316 {
			served <- fmt.Errorf("register procedure = %d, want 316", got)

			return
		}

		if err = replyCall(conn, call, binary.BigEndian.AppendUint32(nil, 7)); err != nil {
			served <- err

			return
		}

		served <- nil

		// Keep the server alive beyond the short operation-session deadline.
		if _, err := readCall(conn); err != nil {
			return
		}
	}()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ch, err := libvirtdomain.New(socket, "qemu:///system").Watch(ctx)
	require.NoError(t, err)
	require.NoError(t, <-served)

	select {
	case <-ch:
		t.Fatal("watch closed before its caller canceled")
	case <-time.After(6 * time.Second):
	}

	cancel()
	watchClosed(t, ch)
}
