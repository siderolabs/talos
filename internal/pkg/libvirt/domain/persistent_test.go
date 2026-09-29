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

	"github.com/stretchr/testify/require"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

func TestOpenPersistentReusesOneSession(t *testing.T) {
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

		if handshakeErr := serveHandshake(conn); handshakeErr != nil {
			served <- handshakeErr

			return
		}

		for range 2 {
			call, callErr := readCall(conn)
			if callErr != nil {
				served <- callErr

				return
			}

			if procedure := binary.BigEndian.Uint32(call[8:12]); procedure != 273 {
				served <- fmt.Errorf("expected CONNECT_LIST_ALL_DOMAINS, got %d", procedure)

				return
			}

			payload := binary.BigEndian.AppendUint32(nil, 0)
			payload = binary.BigEndian.AppendUint32(payload, 0)

			if replyErr := replyCall(conn, call, payload); replyErr != nil {
				served <- replyErr

				return
			}
		}

		served <- nil
	}()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client, err := libvirtdomain.New(socket, "qemu:///system").OpenPersistent(ctx)
	require.NoError(t, err)

	defer client.Close()

	for range 2 {
		domains, listErr := client.Domains()
		require.NoError(t, listErr)
		require.Empty(t, domains)
	}

	require.NoError(t, <-served)
}
