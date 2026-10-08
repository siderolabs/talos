// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/hypervisord"
	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func vncAttach() *machine.VNCRequest {
	return &machine.VNCRequest{Request: &machine.VNCRequest_Attach{Attach: &machine.VNCAttach{Name: "vm1"}}}
}

func vncData(data []byte) *machine.VNCRequest {
	return &machine.VNCRequest{Request: &machine.VNCRequest_Data{Data: data}}
}

func TestVNCProtocol(t *testing.T) {
	for _, tt := range []struct {
		name   string
		frames []*machine.VNCRequest
		code   codes.Code
	}{
		{"data first", []*machine.VNCRequest{vncData(nil)}, codes.InvalidArgument},
		{"empty attach", []*machine.VNCRequest{{Request: &machine.VNCRequest_Attach{Attach: &machine.VNCAttach{}}}}, codes.InvalidArgument},
		{"repeated attach", []*machine.VNCRequest{vncAttach(), vncAttach()}, codes.InvalidArgument},
		{"missing data", []*machine.VNCRequest{vncAttach(), {}}, codes.InvalidArgument},
		{"oversize", []*machine.VNCRequest{vncAttach(), vncData(make([]byte, 64*1024+1))}, codes.ResourceExhausted},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := setup(t)
			vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
			vm.TypedSpec().PowerState = "running"
			vm.TypedSpec().Console.VNC = true // Serial deliberately disabled.
			require.NoError(t, st.Create(t.Context(), vm))
			c := client(t, st, nil, hypervisord.WithVNCConnector(func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
				a, b := net.Pipe()

				t.Cleanup(func() { require.NoError(t, b.Close()) })

				return a, nil
			}))

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			stream, err := c.VNCStream(ctx)
			require.NoError(t, err)

			for _, frame := range tt.frames {
				require.NoError(t, stream.Send(frame))
			}

			_, err = stream.Recv()
			require.Equal(t, tt.code, status.Code(err))
		})
	}
}

func TestVNCPreconditions(t *testing.T) {
	for _, tt := range []struct {
		name, power                string
		exists, enabled, connector bool
		code                       codes.Code
	}{
		{"unconfigured", "running", true, true, false, codes.Unimplemented},
		{"missing", "running", false, true, true, codes.NotFound},
		{"disabled", "running", true, false, true, codes.FailedPrecondition},
		{"stopped", "stopped", true, true, true, codes.FailedPrecondition},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := setup(t)

			if tt.exists {
				vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
				vm.TypedSpec().PowerState = tt.power
				vm.TypedSpec().Console.VNC = tt.enabled
				require.NoError(t, st.Create(t.Context(), vm))
			}

			var options []hypervisord.ServiceOption
			if tt.connector {
				options = append(options, hypervisord.WithVNCConnector(func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
					t.Error("unexpected connector call")

					return nil, status.Error(codes.Internal, "unexpected")
				}))
			}

			c := client(t, st, nil, options...)
			stream, err := c.VNCStream(t.Context())
			require.NoError(t, err)

			if err := stream.Send(vncAttach()); err != nil {
				require.ErrorIs(t, err, io.EOF)
			}

			_, err = stream.Recv()
			require.Equal(t, tt.code, status.Code(err))
		})
	}
}

func TestVNCCancellationClosesBlockedIO(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st := setup(t)
		vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
		vm.TypedSpec().PowerState = "running"
		vm.TypedSpec().Console.VNC = true
		require.NoError(t, st.Create(t.Context(), vm))

		server, peer := net.Pipe()
		defer func() { require.NoError(t, server.Close()) }()
		defer func() { require.NoError(t, peer.Close()) }()

		require.NoError(t, peer.SetReadDeadline(time.Now().Add(5*time.Second)))

		conn := &vncObservedConn{Conn: server}
		c := client(t, st, nil, hypervisord.WithVNCConnector(func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
			return conn, nil
		}))

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		stream, err := c.VNCStream(ctx)
		require.NoError(t, err)
		require.NoError(t, stream.Send(vncAttach()))
		require.NoError(t, stream.Send(vncData([]byte{1, 2, 3})))

		synctest.Wait()
		require.True(t, conn.readStarted.Load(), "cancellation must exercise a blocked server read")
		require.True(t, conn.writeStarted.Load(), "cancellation must exercise a blocked server write")
		cancel()

		_, err = stream.Recv()
		require.Equal(t, codes.Canceled, status.Code(err))
		synctest.Wait()

		var buf [1]byte

		n, err := peer.Read(buf[:])
		require.ErrorIs(t, err, io.EOF, "cancellation must close the pipe, not leave a write pending")
		require.Zero(t, n)
	})
}

type vncObservedConn struct {
	net.Conn

	readStarted  atomic.Bool
	writeStarted atomic.Bool
}

func (c *vncObservedConn) Read(buf []byte) (int, error) {
	c.readStarted.Store(true)

	return c.Conn.Read(buf)
}

func (c *vncObservedConn) Write(buf []byte) (int, error) {
	c.writeStarted.Store(true)

	return c.Conn.Write(buf)
}

func TestVNCRelayAndReservations(t *testing.T) {
	st := setup(t)
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "vm1")
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().Console.VNC = true
	vm.TypedSpec().Console.Serial = true
	require.NoError(t, st.Create(t.Context(), vm))

	var attempts atomic.Int32

	peers := make(chan net.Conn, 4)
	c := client(t, st, func(context.Context, domain.Domain) (io.ReadWriteCloser, error) {
		a, b := net.Pipe()
		peers <- b

		return a, nil
	}, hypervisord.WithVNCConnector(func(_ context.Context, identity domain.Domain) (io.ReadWriteCloser, error) {
		assert.Equal(t, "vm1", identity.Name)

		if attempts.Add(1) == 1 {
			return nil, status.Error(codes.FailedPrecondition, "retry")
		}

		a, b := net.Pipe()
		peers <- b

		return a, nil
	}))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	attach := func() grpc.BidiStreamingClient[machine.VNCRequest, machine.VNCResponse] {
		stream, err := c.VNCStream(ctx)
		require.NoError(t, err)
		require.NoError(t, stream.Send(vncAttach()))

		return stream
	}
	failed := attach()
	_, err := failed.Recv()
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	stream := attach() // A failed open must release its reservation.

	var peer net.Conn
	select {
	case peer = <-peers:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	defer func() { require.NoError(t, peer.Close()) }()

	second := attach()
	_, err = second.Recv()
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	serial, err := c.ConsoleStream(ctx)
	require.NoError(t, err)
	require.NoError(t, serial.Send(&machine.ConsoleRequest{Request: &machine.ConsoleRequest_Attach{Attach: &machine.ConsoleAttach{Name: "vm1"}}}))

	select {
	case serialPeer := <-peers:
		defer func() { require.NoError(t, serialPeer.Close()) }()
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	payload := bytes.Repeat([]byte{0, 255, 128, 1}, 16*1024)
	require.NoError(t, stream.Send(vncData(payload)))
	received := make([]byte, len(payload))

	require.NoError(t, peer.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = io.ReadFull(peer, received)
	require.NoError(t, err)
	require.Equal(t, payload, received)

	written := make(chan error, 1)

	go func() { _, writeErr := peer.Write(payload); written <- writeErr }()

	var output []byte
	for len(output) < len(payload) {
		frame, recvErr := stream.Recv()
		require.NoError(t, recvErr)
		require.LessOrEqual(t, len(frame.Data), 64*1024)
		output = append(output, frame.Data...)
	}

	require.NoError(t, <-written)
	require.Equal(t, payload, output)
	require.NoError(t, stream.CloseSend())
	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)

	next := attach()

	select {
	case nextPeer := <-peers:
		require.NoError(t, nextPeer.Close())
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	_, err = next.Recv()
	require.ErrorIs(t, err, io.EOF)
}
