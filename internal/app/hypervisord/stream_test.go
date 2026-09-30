// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func consoleClient(t *testing.T) (machine.HypervisorServiceClient, <-chan net.Conn) {
	t.Helper()

	st := setup(t)
	addVM(t, st, true, "running")

	opened := make(chan net.Conn, 4)
	c := client(t, st, func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
		host, guest := net.Pipe()
		opened <- guest

		return host, nil
	})

	return c, opened
}

func guestConnection(t *testing.T, opened <-chan net.Conn) net.Conn {
	t.Helper()

	select {
	case guest := <-opened:
		require.NoError(t, guest.SetDeadline(time.Now().Add(5*time.Second)))
		t.Cleanup(func() { require.NoError(t, guest.Close()) })

		return guest
	case <-time.After(5 * time.Second):
		t.Fatal("console did not open")

		return nil
	}
}

func TestRawBytesAndInputBound(t *testing.T) {
	c, opened := consoleClient(t)
	stream, _ := openStream(t, c)
	require.NoError(t, stream.Send(attach("vm1")))
	guest := guestConnection(t, opened)
	payload := bytes.Repeat([]byte{0, 255, '\r', '\n'}, 16*1024)

	require.NoError(t, stream.Send(input(nil)))
	require.NoError(t, stream.Send(input(payload)))
	got := make([]byte, len(payload))
	_, err := io.ReadFull(guest, got)
	require.NoError(t, err)
	require.Equal(t, payload, got)

	writeDone := make(chan error, 1)

	go func() {
		_, writeErr := guest.Write(payload)
		writeDone <- writeErr
	}()

	var output []byte

	for len(output) < len(payload) {
		response, recvErr := stream.Recv()
		require.NoError(t, recvErr)

		output = append(output, response.GetStdoutData()...)
	}

	require.NoError(t, <-writeDone)
	require.Equal(t, payload, output)
	require.NoError(t, stream.CloseSend())
	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func TestInvalidInputFrames(t *testing.T) {
	for _, test := range []struct {
		name    string
		request *machine.ConsoleRequest
		code    codes.Code
	}{
		{name: "reattach", request: attach("vm1"), code: codes.InvalidArgument},
		{name: "empty", request: &machine.ConsoleRequest{}, code: codes.InvalidArgument},
		{name: "oversize", request: input(make([]byte, 64*1024+1)), code: codes.ResourceExhausted},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, opened := consoleClient(t)
			stream, _ := openStream(t, c)
			require.NoError(t, stream.Send(attach("vm1")))
			guest := guestConnection(t, opened)
			require.NoError(t, stream.Send(test.request))
			_, err := stream.Recv()
			require.Equal(t, test.code, status.Code(err))
			_, err = guest.Read(make([]byte, 1))
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

func TestExclusiveAttachAndRelease(t *testing.T) {
	for _, ending := range []string{"cancel", "input EOF", "guest EOF"} {
		t.Run(ending, func(t *testing.T) {
			c, opened := consoleClient(t)
			first, cancel := openStream(t, c)
			require.NoError(t, first.Send(attach("vm1")))
			guest := guestConnection(t, opened)
			second, _ := openStream(t, c)
			require.NoError(t, second.Send(attach("vm1")))
			_, err := second.Recv()
			require.Equal(t, codes.AlreadyExists, status.Code(err))

			switch ending {
			case "cancel":
				cancel()
			case "input EOF":
				require.NoError(t, first.CloseSend())
			case "guest EOF":
				require.NoError(t, guest.Close())
			}

			_, err = first.Recv()
			if ending == "cancel" {
				require.Equal(t, codes.Canceled, status.Code(err))
			} else {
				require.ErrorIs(t, err, io.EOF)
			}

			// A canceled client can finish before the server releases the slot.
			// Retry only AlreadyExists, with every attempt bounded by its RPC context.
			deadline := time.Now().Add(5 * time.Second)

			for {
				retry, retryCancel := openStream(t, c)
				require.NoError(t, retry.Send(attach("vm1")))
				require.NoError(t, retry.CloseSend())

				_, retryErr := retry.Recv()

				retryCancel()

				if status.Code(retryErr) != codes.AlreadyExists {
					require.ErrorIs(t, retryErr, io.EOF)

					break
				}

				require.True(t, time.Now().Before(deadline), "console slot was not released")
				time.Sleep(time.Millisecond)
			}

			select {
			case next := <-opened:
				require.NoError(t, next.Close())
			case <-time.After(5 * time.Second):
				t.Fatal("console was not reopened")
			}
		})
	}
}
