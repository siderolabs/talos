// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package vmconsole_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siderolabs/talos/internal/pkg/vmconsole"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

type consoleServer struct {
	machine.UnimplementedHypervisorServiceServer
	requests chan *machine.ConsoleRequest
	fail     error
	finish   bool
}

type signalWriter struct {
	bytes.Buffer
	wrote chan struct{}
}

type scheduledConsoleStream struct {
	grpc.ClientStream
	done         <-chan struct{}
	received     chan struct{}
	sent         chan struct{}
	releaseRecv  <-chan struct{}
	observedSend chan<- error
	observedRecv chan<- error
}

type scheduledInitialAttachStream struct {
	grpc.ClientStream
	done         <-chan struct{}
	releaseRecv  <-chan struct{}
	observedHead chan<- error
	observedSend chan<- error
	observedRecv chan<- error
}

type initialAttachSendErrorStream struct {
	grpc.ClientStream
	err error
}

func (s *scheduledConsoleStream) SendMsg(message any) error {
	request := message.(*machine.ConsoleRequest)
	if request.GetAttach() != nil {
		return s.ClientStream.SendMsg(message)
	}

	select {
	case <-s.received:
	case <-s.done:
		return context.Canceled
	}

	err := s.ClientStream.SendMsg(message)
	s.observedSend <- err

	close(s.sent)

	return err
}

func (s *scheduledConsoleStream) RecvMsg(message any) error {
	err := s.ClientStream.RecvMsg(message)
	s.observedRecv <- err

	close(s.received)

	select {
	case <-s.sent:
	case <-s.done:
		return err
	}

	select {
	case <-s.releaseRecv:
	case <-s.done:
	}

	return err
}

func (s *scheduledInitialAttachStream) SendMsg(message any) error {
	if request := message.(*machine.ConsoleRequest); request.GetAttach() != nil {
		_, err := s.ClientStream.Header()
		s.observedHead <- err
	}

	err := s.ClientStream.SendMsg(message)
	s.observedSend <- err

	return err
}

func (s *scheduledInitialAttachStream) RecvMsg(message any) error {
	err := s.ClientStream.RecvMsg(message)
	s.observedRecv <- err

	select {
	case <-s.releaseRecv:
	case <-s.done:
	}

	return err
}

func (s *initialAttachSendErrorStream) SendMsg(message any) error {
	if request := message.(*machine.ConsoleRequest); request.GetAttach() != nil {
		return s.err
	}

	return s.ClientStream.SendMsg(message)
}

func (w *signalWriter) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data)

	select {
	case <-w.wrote:
	default:
		close(w.wrote)
	}

	return n, err
}

func (s *consoleServer) ConsoleStream(stream grpc.BidiStreamingServer[machine.ConsoleRequest, machine.ConsoleResponse]) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}

	s.requests <- first

	if s.fail != nil || s.finish {
		return s.fail
	}

	if err := stream.Send(&machine.ConsoleResponse{StdoutData: []byte{0, 27, 0xff, '\n'}}); err != nil {
		return err
	}

	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		select {
		case s.requests <- req:
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func newConsoleClient(t *testing.T, server *consoleServer) machine.HypervisorServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	machine.RegisterHypervisorServiceServer(grpcServer, server)

	serveDone := make(chan error, 1)
	go func() { serveDone <- grpcServer.Serve(listener) }()

	t.Cleanup(func() {
		grpcServer.Stop()
		assert.NoError(t, <-serveDone)
		assert.NoError(t, listener.Close())
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	return machine.NewHypervisorServiceClient(conn)
}

func inputPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	input, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, input.Close())

		if err := writer.Close(); err != nil {
			assert.ErrorIs(t, err, os.ErrClosed)
		}
	})

	return input, writer
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("console operation timed out")

		var zero T

		return zero
	}
}

func TestAttachOutputAndLocalDetach(t *testing.T) {
	server := &consoleServer{requests: make(chan *machine.ConsoleRequest, 4)}
	client := newConsoleClient(t, server)
	input, writer := inputPipe(t)
	output := &signalWriter{wrote: make(chan struct{})}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- vmconsole.Run(ctx, client, "guest", input, output) }()

	req := receive(t, server.requests)
	assert.Equal(t, "guest", req.GetAttach().GetName())
	assert.IsType(t, &machine.ConsoleRequest_Attach{}, req.Request)
	receive(t, output.wrote)

	_, err := writer.Write([]byte{'a', 3, 0})
	require.NoError(t, err)
	req = receive(t, server.requests)
	assert.Equal(t, []byte{'a', 3, 0}, req.GetStdinData())

	_, err = writer.Write([]byte{29, 'z'})
	require.NoError(t, err)
	require.NoError(t, receive(t, done))
	assert.Equal(t, []byte{0, 27, 0xff, '\n'}, output.Bytes())

	select {
	case req := <-server.requests:
		t.Errorf("unexpected request after detach: %v", req)
	default:
	}
}

func TestEOFDetaches(t *testing.T) {
	server := &consoleServer{requests: make(chan *machine.ConsoleRequest, 2)}
	client := newConsoleClient(t, server)
	input, writer := inputPipe(t)
	require.NoError(t, writer.Close())

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	require.NoError(t, vmconsole.Run(ctx, client, "guest", input, io.Discard))
}

func TestCancellationWhileStdinIdle(t *testing.T) {
	server := &consoleServer{requests: make(chan *machine.ConsoleRequest, 2)}
	client := newConsoleClient(t, server)
	input, _ := inputPipe(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- vmconsole.Run(ctx, client, "guest", input, io.Discard) }()

	receive(t, server.requests)
	cancel()
	assert.ErrorIs(t, receive(t, done), context.Canceled)
}

func TestShortOutputWriteReturnsError(t *testing.T) {
	server := &consoleServer{requests: make(chan *machine.ConsoleRequest, 2)}
	client := newConsoleClient(t, server)
	input, _ := inputPipe(t)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := vmconsole.Run(ctx, client, "guest", input, shortWriter{})
	assert.ErrorIs(t, err, io.ErrShortWrite)
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

func TestRemoteFailureWhileStdinIdle(t *testing.T) {
	server := &consoleServer{
		requests: make(chan *machine.ConsoleRequest, 2),
		fail:     status.Error(codes.FailedPrecondition, "console occupied"),
	}
	client := newConsoleClient(t, server)
	input, _ := inputPipe(t)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := vmconsole.Run(ctx, client, "guest", input, io.Discard)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRemoteFailureWhileSendingPreservesStatus(t *testing.T) {
	for _, code := range []codes.Code{codes.FailedPrecondition, codes.PermissionDenied, codes.Unavailable} {
		t.Run(code.String(), func(t *testing.T) {
			listener := bufconn.Listen(1024 * 1024)
			grpcServer := grpc.NewServer()
			machine.RegisterHypervisorServiceServer(grpcServer, &consoleServer{
				requests: make(chan *machine.ConsoleRequest, 2),
				fail:     status.Error(code, "server rejected console"),
			})

			serveDone := make(chan error, 1)
			go func() { serveDone <- grpcServer.Serve(listener) }()

			t.Cleanup(func() {
				grpcServer.Stop()
				assert.NoError(t, <-serveDone)
				assert.NoError(t, listener.Close())
			})

			sendErr := make(chan error, 1)
			recvErr := make(chan error, 1)
			releaseRecv := make(chan struct{})

			conn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return listener.DialContext(ctx)
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithStreamInterceptor(func(
					ctx context.Context,
					desc *grpc.StreamDesc,
					connection *grpc.ClientConn,
					method string,
					streamer grpc.Streamer,
					opts ...grpc.CallOption,
				) (grpc.ClientStream, error) {
					stream, streamErr := streamer(ctx, desc, connection, method, opts...)
					if streamErr != nil {
						return nil, streamErr
					}

					return &scheduledConsoleStream{
						ClientStream: stream,
						done:         ctx.Done(),
						received:     make(chan struct{}),
						sent:         make(chan struct{}),
						releaseRecv:  releaseRecv,
						observedSend: sendErr,
						observedRecv: recvErr,
					}, nil
				}),
			)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, conn.Close()) })

			input, writer := inputPipe(t)
			_, err = writer.WriteString("x")
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				done <- vmconsole.Run(ctx, machine.NewHypervisorServiceClient(conn), "guest", input, io.Discard)
			}()

			actualSend := receive(t, sendErr)
			actualRecv := receive(t, recvErr)
			require.ErrorIs(t, actualSend, io.EOF)
			require.Equal(t, code, status.Code(actualRecv))

			var (
				runErr         error
				completedEarly bool
			)

			select {
			case runErr = <-done:
				completedEarly = true
			case <-time.After(100 * time.Millisecond):
			}

			close(releaseRecv)

			if !completedEarly {
				runErr = receive(t, done)
			}

			assert.False(t, completedEarly, "Run completed before the delayed receive result was released")
			assert.Equal(t, code, status.Code(runErr), "Run discarded the server's terminal status")
		})
	}
}

func TestInitialAttachSendEOFPreservesTerminalStatus(t *testing.T) {
	for _, test := range []struct {
		name       string
		serverErr  error
		statusCode codes.Code
	}{
		{
			name:       "permission denied",
			serverErr:  status.Error(codes.PermissionDenied, "console access denied"),
			statusCode: codes.PermissionDenied,
		},
		{
			name:       "unavailable",
			serverErr:  status.Error(codes.Unavailable, "console service unavailable"),
			statusCode: codes.Unavailable,
		},
		{
			name:       "failed precondition",
			serverErr:  status.Error(codes.FailedPrecondition, "console occupied"),
			statusCode: codes.FailedPrecondition,
		},
		{
			name: "normal EOF",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener := bufconn.Listen(1024 * 1024)
			grpcServer := grpc.NewServer(grpc.StreamInterceptor(func(
				_ any,
				_ grpc.ServerStream,
				_ *grpc.StreamServerInfo,
				_ grpc.StreamHandler,
			) error {
				return test.serverErr
			}))
			machine.RegisterHypervisorServiceServer(grpcServer, &consoleServer{})

			serveDone := make(chan error, 1)
			go func() { serveDone <- grpcServer.Serve(listener) }()

			t.Cleanup(func() {
				grpcServer.Stop()
				assert.NoError(t, <-serveDone)
				assert.NoError(t, listener.Close())
			})

			headerErr := make(chan error, 1)
			sendErr := make(chan error, 1)
			recvErr := make(chan error, 1)
			releaseRecv := make(chan struct{})
			release := func() {
				select {
				case <-releaseRecv:
				default:
					close(releaseRecv)
				}
			}
			t.Cleanup(release)

			conn, err := grpc.NewClient("passthrough:///bufnet",
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return listener.DialContext(ctx)
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithStreamInterceptor(func(
					ctx context.Context,
					desc *grpc.StreamDesc,
					connection *grpc.ClientConn,
					method string,
					streamer grpc.Streamer,
					opts ...grpc.CallOption,
				) (grpc.ClientStream, error) {
					stream, streamErr := streamer(ctx, desc, connection, method, opts...)
					if streamErr != nil {
						return nil, streamErr
					}

					return &scheduledInitialAttachStream{
						ClientStream: stream,
						done:         ctx.Done(),
						releaseRecv:  releaseRecv,
						observedHead: headerErr,
						observedSend: sendErr,
						observedRecv: recvErr,
					}, nil
				}),
			)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, conn.Close()) })

			input, _ := inputPipe(t)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				done <- vmconsole.Run(ctx, machine.NewHypervisorServiceClient(conn), "guest", input, io.Discard)
			}()

			require.NoError(t, receive(t, headerErr))
			require.ErrorIs(t, receive(t, sendErr), io.EOF)

			actualRecv := receive(t, recvErr)
			if test.serverErr == nil {
				require.ErrorIs(t, actualRecv, io.EOF)
			} else {
				require.Equal(t, test.statusCode, status.Code(actualRecv))
			}

			select {
			case runErr := <-done:
				t.Fatalf("Run completed before the receive result was released: %v", runErr)
			case <-time.After(100 * time.Millisecond):
			}

			release()

			runErr := receive(t, done)
			if test.serverErr == nil {
				require.NoError(t, runErr)
			} else {
				require.Equal(t, test.statusCode, status.Code(runErr))
			}
		})
	}
}

func TestInitialAttachPreservesNonEOFSendError(t *testing.T) {
	wantErr := errors.New("attach transport failed")
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	machine.RegisterHypervisorServiceServer(grpcServer, &consoleServer{requests: make(chan *machine.ConsoleRequest, 1)})

	serveDone := make(chan error, 1)
	go func() { serveDone <- grpcServer.Serve(listener) }()

	t.Cleanup(func() {
		grpcServer.Stop()
		assert.NoError(t, <-serveDone)
		assert.NoError(t, listener.Close())
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(func(
			ctx context.Context,
			desc *grpc.StreamDesc,
			connection *grpc.ClientConn,
			method string,
			streamer grpc.Streamer,
			opts ...grpc.CallOption,
		) (grpc.ClientStream, error) {
			stream, streamErr := streamer(ctx, desc, connection, method, opts...)
			if streamErr != nil {
				return nil, streamErr
			}

			return &initialAttachSendErrorStream{ClientStream: stream, err: wantErr}, nil
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	input, _ := inputPipe(t)
	err = vmconsole.Run(t.Context(), machine.NewHypervisorServiceClient(conn), "guest", input, io.Discard)
	require.ErrorIs(t, err, wantErr)
}

func TestDeadlineWhileStdinIdle(t *testing.T) {
	server := &consoleServer{requests: make(chan *machine.ConsoleRequest, 2)}
	client := newConsoleClient(t, server)
	input, _ := inputPipe(t)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- vmconsole.Run(ctx, client, "guest", input, io.Discard) }()

	receive(t, server.requests)
	assert.ErrorIs(t, receive(t, done), context.DeadlineExceeded)
}

func TestRemoteEOFWhileStdinIdle(t *testing.T) {
	server := &consoleServer{
		requests: make(chan *machine.ConsoleRequest, 2),
		finish:   true,
	}
	client := newConsoleClient(t, server)
	input, _ := inputPipe(t)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	require.NoError(t, vmconsole.Run(ctx, client, "guest", input, io.Discard))
}
