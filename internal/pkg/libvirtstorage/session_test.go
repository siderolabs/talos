// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package libvirtstorage_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/libvirtstorage"
)

func TestSessionDeadlineClosesHandshakeReadStall(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		raw, server := net.Pipe()
		defer server.Close() //nolint:errcheck
		defer raw.Close()    //nolint:errcheck

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		done := make(chan error, 1)

		go func() {
			_, err := libvirtstorage.OpenConn(ctx, raw, cancel)
			done <- err
		}()
		// Accept the complete authentication request: its write has succeeded, but
		// the daemon never sends a reply. Caller context remains live throughout.
		call, err := readCall(server)
		require.NoError(t, err)
		require.EqualValues(t, 66, binary.BigEndian.Uint32(call[8:12]))

		synctest.Sleep(time.Second)

		select {
		case err = <-done:
			require.Error(t, err)
		default:
			t.Error("session deadline did not close transport after a successful write and stalled reply")
			require.NoError(t, raw.Close())
			<-done
		}

		require.NoError(t, t.Context().Err(), "session expiry must not cancel the caller")
	})
}

// The remote protocol has a length prefix and six uint32 header fields:
// program, version, procedure, message type, serial, status.
func readCall(conn net.Conn) ([]byte, error) {
	var size uint32
	if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
		return nil, err
	}

	if size < 28 || size > 1<<20 {
		return nil, fmt.Errorf("invalid test RPC size %d", size)
	}

	call := make([]byte, size-4)
	_, err := io.ReadFull(conn, call)

	return call, err
}

func replyCall(conn net.Conn, call, payload []byte) error {
	reply := binary.BigEndian.AppendUint32(nil, uint32(28+len(payload)))
	reply = append(reply, call[:24]...)
	binary.BigEndian.PutUint32(reply[16:20], 1) // REMOTE_REPLY
	binary.BigEndian.PutUint32(reply[24:28], 0) // REMOTE_OK
	reply = append(reply, payload...)
	_, err := conn.Write(reply)

	return err
}

func serveHandshake(conn net.Conn) error {
	for _, procedure := range []uint32{66, 1} { // AUTH_LIST, CONNECT_OPEN
		call, err := readCall(conn)
		if err != nil {
			return err
		}

		if actual := binary.BigEndian.Uint32(call[8:12]); actual != procedure {
			return fmt.Errorf("expected procedure %d, got %d", procedure, actual)
		}

		var payload []byte
		if procedure == 66 {
			payload = make([]byte, 4)
		} // empty authentication list

		if err = replyCall(conn, call, payload); err != nil {
			return err
		}
	}

	return nil
}

func TestConnectedSessionReadStall(t *testing.T) {
	t.Parallel()

	for _, cancelCaller := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelCaller), func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				raw, server := net.Pipe()
				defer server.Close() //nolint:errcheck
				defer raw.Close()    //nolint:errcheck

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				accepted := make(chan error, 1)

				go func() {
					if err := serveHandshake(server); err != nil {
						accepted <- err

						return
					}

					_, err := readCall(server)
					accepted <- err // accept the storage RPC but never reply
				}()

				sessionCtx, cancelSession := context.WithTimeout(ctx, time.Second)
				defer cancelSession()

				client, err := libvirtstorage.OpenConn(sessionCtx, raw, cancelSession)
				require.NoError(t, err)

				defer client.Close()

				done := make(chan error, 1)

				go func() { _, rpcErr := client.Pools(); done <- rpcErr }()

				require.NoError(t, <-accepted)

				canceledAt := time.Now()

				if cancelCaller {
					cancel()
					synctest.Wait()
				} else {
					synctest.Sleep(time.Second)
				}

				select {
				case err = <-done:
					require.Error(t, err)

					if cancelCaller {
						require.Equal(t, canceledAt, time.Now(), "caller cancellation must not wait for the session deadline")
					}
				default:
					t.Error("connected RPC did not return after session expiry/cancellation")
					require.NoError(t, raw.Close())
					<-done
				}
			})
		})
	}
}

func TestSessionGracefulClose(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		raw, server := net.Pipe()
		defer server.Close() //nolint:errcheck
		defer raw.Close()    //nolint:errcheck

		done := make(chan error, 1)

		go func() {
			if err := serveHandshake(server); err != nil {
				done <- err

				return
			}

			call, err := readCall(server)
			if err != nil {
				done <- fmt.Errorf("missing CONNECT_CLOSE: %w", err)

				return
			}

			if procedure := binary.BigEndian.Uint32(call[8:12]); procedure != 2 {
				done <- fmt.Errorf("expected CONNECT_CLOSE, got %d", procedure)

				return
			}

			if err = replyCall(server, call, nil); err != nil {
				done <- err

				return
			}

			_, err = io.Copy(io.Discard, server)
			done <- err
		}()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		client, err := libvirtstorage.OpenConn(ctx, raw, cancel)
		require.NoError(t, err)
		client.Close()
		require.NoError(t, <-done)
	})
}

func TestSessionGracefulCloseReadStallIsBounded(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		raw, server := net.Pipe()
		defer server.Close() //nolint:errcheck
		defer raw.Close()    //nolint:errcheck

		accepted := make(chan error, 1)

		go func() {
			if err := serveHandshake(server); err != nil {
				accepted <- err

				return
			}

			call, err := readCall(server)
			if err == nil && binary.BigEndian.Uint32(call[8:12]) != 2 {
				err = errors.New("expected CONNECT_CLOSE")
			}

			accepted <- err // never acknowledge CONNECT_CLOSE
		}()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		client, err := libvirtstorage.OpenConn(ctx, raw, cancel)
		require.NoError(t, err)

		done := make(chan struct{})

		go func() { client.Close(); close(done) }()

		require.NoError(t, <-accepted)

		synctest.Sleep(time.Second)

		select {
		case <-done:
		default:
			t.Error("graceful close waited beyond the session deadline")
			require.NoError(t, raw.Close())
			<-done
		}
	})
}
