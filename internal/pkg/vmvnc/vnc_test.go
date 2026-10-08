// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vmvnc_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siderolabs/talos/internal/pkg/vmvnc"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

const chunkSize = 64 * 1024

type testServer struct {
	machine.UnimplementedHypervisorServiceServer
	sessions chan machine.HypervisorService_VNCStreamServer
	reject   bool
}

func (s *testServer) VNCStream(stream machine.HypervisorService_VNCStreamServer) error {
	request, err := stream.Recv()
	if err != nil {
		return err
	}

	if request.GetAttach().GetName() != "test" {
		return status.Error(codes.InvalidArgument, "missing attachment")
	}

	if s.reject {
		return status.Error(codes.PermissionDenied, "denied test attachment")
	}

	s.sessions <- stream

	if err := stream.Send(&machine.VNCResponse{Data: []byte("RFB 003.008\n")}); err != nil {
		return err
	}

	for {
		request, err := stream.Recv()
		if err != nil {
			return err
		}

		if err = stream.Send(&machine.VNCResponse{Data: request.GetData()}); err != nil {
			return err
		}
	}
}

func setup(t *testing.T, reject bool) (net.Listener, <-chan error, <-chan error, *testServer, context.CancelFunc) {
	t.Helper()

	transport := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	service := &testServer{sessions: make(chan machine.HypervisorService_VNCStreamServer, 8), reject: reject}

	machine.RegisterHypervisorServiceServer(server, service)
	go func() { assert.NoError(t, server.Serve(transport)) }()

	t.Cleanup(server.Stop)

	client, err := grpc.NewClient("passthrough:///vnc-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return transport.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	listener, err := vmvnc.Listen(t.Context(), 0)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	reports := make(chan error, 8)

	go func() {
		done <- vmvnc.Serve(ctx, machine.NewHypervisorServiceClient(client), "test", listener, func(err error) { reports <- err })
	}()

	return listener, done, reports, service, cancel
}

func viewer(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()

	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			assert.ErrorIs(t, err, net.ErrClosed)
		}
	})

	return conn
}

func waitDone(t *testing.T, done <-chan error) {
	t.Helper()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not stop")
	}
}

func TestListen(t *testing.T) {
	for _, port := range []int{-1, 65536} {
		_, err := vmvnc.Listen(t.Context(), port)
		require.Error(t, err)
	}

	listener, err := vmvnc.Listen(t.Context(), 0)
	require.NoError(t, err)

	addr := listener.Addr().(*net.TCPAddr)
	require.True(t, addr.IP.IsLoopback())
	require.Positive(t, addr.Port)
	_, err = vmvnc.Listen(t.Context(), addr.Port)
	require.Error(t, err)
	require.NoError(t, listener.Close())

	fixed, err := vmvnc.Listen(t.Context(), addr.Port)
	require.NoError(t, err)
	require.NoError(t, fixed.Close())
}

func TestRelayAndSingleViewer(t *testing.T) {
	listener, done, _, service, cancel := setup(t, false)
	conn := viewer(t, listener)
	greeting := make([]byte, 12)
	_, err := io.ReadFull(conn, greeting)
	require.NoError(t, err)
	require.Equal(t, "RFB 003.008\n", string(greeting))
	<-service.sessions

	payload := bytes.Repeat([]byte{0, 255, 128, 13}, 32768)
	sent := make(chan error, 1)

	go func() { _, err := conn.Write(payload); sent <- err }()

	actual := make([]byte, len(payload))
	_, err = io.ReadFull(conn, actual)
	require.NoError(t, err)
	require.NoError(t, <-sent)
	require.Equal(t, payload, actual)
	second := viewer(t, listener)
	_, err = second.Read(make([]byte, 1))
	require.Error(t, err)

	select {
	case <-service.sessions:
		t.Fatal("second viewer opened a remote stream")
	default:
	}

	cancel()
	waitDone(t, done)

	_, err = conn.Read(make([]byte, 1))
	require.Error(t, err)
}

func TestReconnect(t *testing.T) {
	listener, done, _, service, cancel := setup(t, false)
	first := viewer(t, listener)
	_, err := io.ReadFull(first, make([]byte, 12))
	require.NoError(t, err)

	session := <-service.sessions

	require.NoError(t, first.Close())

	select {
	case <-session.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("remote stream not canceled")
	}
	// The remote cancellation can precede the local worker releasing its slot.
	require.Eventually(t, func() bool {
		conn, dialErr := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", listener.Addr().String())
		if dialErr != nil {
			return false
		}
		defer conn.Close() //nolint:errcheck

		require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
		_, readErr := io.ReadFull(conn, make([]byte, 12))

		return readErr == nil
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	waitDone(t, done)
}

func TestFailedAttachVisible(t *testing.T) {
	listener, done, reports, _, cancel := setup(t, true)
	conn := viewer(t, listener)
	_, err := conn.Read(make([]byte, 1))
	require.Error(t, err)

	select {
	case err := <-reports:
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.ErrorContains(t, err, "denied test attachment")
	case <-time.After(5 * time.Second):
		t.Fatal("remote attach error was hidden")
	}

	cancel()
	waitDone(t, done)
}

type stalledClient struct {
	machine.HypervisorServiceClient
	opened chan struct{}
	setup  bool
}

func (c *stalledClient) VNCStream(ctx context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[machine.VNCRequest, machine.VNCResponse], error) {
	close(c.opened)

	if c.setup {
		<-ctx.Done()

		return nil, ctx.Err()
	}

	return &stalledStream{ctx: ctx}, nil
}

//nolint:containedctx // Implements a gRPC client stream's cancellation behavior.
type stalledStream struct {
	machine.HypervisorService_VNCStreamClient
	ctx context.Context
}

func (s *stalledStream) Send(request *machine.VNCRequest) error {
	if request.GetAttach() != nil {
		return nil
	}

	<-s.ctx.Done()

	return s.ctx.Err()
}

func (s *stalledStream) Recv() (*machine.VNCResponse, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}

	return &machine.VNCResponse{Data: make([]byte, chunkSize)}, nil
}

func TestCancelBlockedSetupAndRelay(t *testing.T) {
	for _, setup := range []bool{true, false} {
		t.Run(map[bool]string{true: "setup", false: "relay"}[setup], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			local, remote := net.Pipe()
			defer remote.Close() //nolint:errcheck

			client := &stalledClient{opened: make(chan struct{}), setup: setup}

			done := make(chan error, 1)
			go func() { done <- vmvnc.Relay(ctx, client, "test", local) }()

			<-client.opened

			if !setup {
				// Viewer never reads the response, and input Send blocks too.
				_, err := remote.Write([]byte("input"))
				require.NoError(t, err)
			}

			cancel()

			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("relay workers did not stop")
			}
		})
	}
}

type partialConn struct {
	net.Conn
	data bytes.Buffer
}

func (c *partialConn) Write(data []byte) (int, error) {
	return c.data.Write(data[:min(3, len(data))])
}

type outputStream struct {
	machine.HypervisorService_VNCStreamClient
	data []byte
}

func (s *outputStream) Recv() (*machine.VNCResponse, error) {
	if s.data == nil {
		return nil, io.EOF
	}

	data := s.data
	s.data = nil

	return &machine.VNCResponse{Data: data}, nil
}

func TestOutputPartialWritesAndBound(t *testing.T) {
	payload := []byte{0, 255, 1, 128, 13, 10, 4, 5}
	conn := &partialConn{}
	require.ErrorIs(t, vmvnc.ForwardOutput(conn, &outputStream{data: payload}), io.EOF)
	require.Equal(t, payload, conn.data.Bytes())
	require.ErrorContains(t, vmvnc.ForwardOutput(conn, &outputStream{data: make([]byte, chunkSize+1)}), "exceeds 64 KiB")
}

func TestCancelAccept(t *testing.T) {
	listener, done, _, service, cancel := setup(t, false)
	require.NotNil(t, listener)
	require.Empty(t, service.sessions)
	cancel()
	waitDone(t, done)
}
