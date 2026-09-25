// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package libvirt_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/libvirt"
)

func TestClientUsesConfiguredDaemonSockets(t *testing.T) {
	t.Parallel()

	storageSocket := t.TempDir() + "/storage.sock"
	domainSocket := t.TempDir() + "/qemu.sock"
	client := libvirt.NewWithConfig(libvirt.Config{
		StorageSocket: storageSocket,
		StorageURI:    "storage:///system",
		DomainSocket:  domainSocket,
		DomainURI:     "qemu:///system",
	})

	_, err := client.Storage(t.Context())
	require.ErrorContains(t, err, storageSocket)

	_, err = client.Domain(t.Context())
	require.ErrorContains(t, err, domainSocket)
}

func TestClientUsesConfiguredDaemonURIs(t *testing.T) {
	t.Parallel()

	storageSocket := filepath.Join(t.TempDir(), "storage.sock")
	domainSocket := filepath.Join(t.TempDir(), "qemu.sock")
	storageURI := "storage:///configured"
	domainURI := "qemu:///configured"
	storageDone := listenForURI(t, storageSocket, storageURI)
	domainDone := listenForURI(t, domainSocket, domainURI)

	client := libvirt.NewWithConfig(libvirt.Config{
		StorageSocket: storageSocket,
		StorageURI:    storageURI,
		DomainSocket:  domainSocket,
		DomainURI:     domainURI,
	})

	storageClient, err := client.Storage(t.Context())
	require.NoError(t, err)
	storageClient.Close()
	require.NoError(t, <-storageDone)

	domainClient, err := client.Domain(t.Context())
	require.NoError(t, err)
	domainClient.Close()
	require.NoError(t, <-domainDone)
}

func listenForURI(t *testing.T, socket, uri string) <-chan error {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	done := make(chan error, 1)

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr

			return
		}
		defer conn.Close() //nolint:errcheck

		done <- serveConfiguredSession(conn, uri)
	}()

	return done
}

func serveConfiguredSession(conn net.Conn, uri string) error {
	for _, procedure := range []uint32{66, 1, 2} { // AUTH_LIST, CONNECT_OPEN, CONNECT_CLOSE
		var size uint32
		if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
			return err
		}

		if size < 28 || size > 1<<20 {
			return fmt.Errorf("invalid libvirt RPC size %d", size)
		}

		call := make([]byte, size-4)
		if _, err := io.ReadFull(conn, call); err != nil {
			return err
		}

		if actual := binary.BigEndian.Uint32(call[8:12]); actual != procedure {
			return fmt.Errorf("expected libvirt procedure %d, got %d", procedure, actual)
		}

		if err := checkConfiguredURI(procedure, call, uri); err != nil {
			return err
		}

		var payload []byte
		if procedure == 66 {
			payload = make([]byte, 4)
		}

		reply := binary.BigEndian.AppendUint32(nil, uint32(28+len(payload)))
		reply = append(reply, call[:24]...)
		binary.BigEndian.PutUint32(reply[16:20], 1) // REMOTE_REPLY
		binary.BigEndian.PutUint32(reply[24:28], 0) // REMOTE_OK
		reply = append(reply, payload...)

		if _, err := conn.Write(reply); err != nil {
			return err
		}
	}

	return nil
}

func checkConfiguredURI(procedure uint32, call []byte, uri string) error {
	if procedure != 1 {
		return nil
	}

	if !bytes.Contains(call[24:], []byte(uri)) {
		return fmt.Errorf("CONNECT_OPEN did not contain configured URI %q", uri)
	}

	return nil
}

func TestClientComposesIndependentDaemonSessions(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	client := libvirt.New()

	cancel()

	poolClient, err := client.Storage(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, poolClient)

	domainClient, err := client.Domain(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, domainClient)
}
