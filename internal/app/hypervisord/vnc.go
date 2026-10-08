// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisord

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// VNCStream relays opaque RFB bytes with an independent, exclusive reservation.
//
//nolint:gocyclo // Reservation and stream cleanup must cover every attachment exit.
func (s *Service) VNCStream(stream grpc.BidiStreamingServer[machine.VNCRequest, machine.VNCResponse]) (retErr error) {
	if s.openVNC == nil {
		return status.Error(codes.Unimplemented, "VNC connector is not configured")
	}

	first, err := stream.Recv()
	if err != nil {
		return err
	}

	if first.GetAttach().GetName() == "" {
		return status.Error(codes.InvalidArgument, "first VNC frame must attach a named virtual machine")
	}

	identity, err := s.attachmentIdentity(stream.Context(), first.GetAttach().GetName(), true)
	if err != nil {
		return err
	}

	s.mu.Lock()

	_, occupied := s.vncAttached[identity.Name]
	if !occupied {
		s.vncAttached[identity.Name] = struct{}{}
	}
	s.mu.Unlock()

	if occupied {
		return status.Error(codes.AlreadyExists, "virtual machine VNC is already attached")
	}

	defer func() {
		s.mu.Lock()
		delete(s.vncAttached, identity.Name)
		s.mu.Unlock()
	}()

	conn, err := s.openVNC(stream.Context(), identity)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return status.FromContextError(err).Err()
		}

		return openConsoleError(err)
	}

	var (
		closeOnce sync.Once
		closeErr  error
	)

	closeConnection := func() { closeOnce.Do(func() { closeErr = conn.Close() }) }
	defer func() {
		closeConnection()

		if retErr == nil && closeErr != nil {
			retErr = fmt.Errorf("close VNC connection: %w", closeErr)
		}
	}()

	stop := context.AfterFunc(stream.Context(), closeConnection)
	defer stop()

	// No data queues: both directions propagate destination backpressure.
	// Returning closes the connection and gRPC cancels blocked Send/Recv calls.
	done := make(chan error, 2)
	go func() { done <- sendVNCOutput(stream, conn) }()
	go func() { done <- receiveVNCInput(stream, conn) }()

	select {
	case <-stream.Context().Done():
		return status.FromContextError(stream.Context().Err()).Err()
	case err = <-done:
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}
}

func sendVNCOutput(stream grpc.BidiStreamingServer[machine.VNCRequest, machine.VNCResponse], conn io.Reader) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := conn.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&machine.VNCResponse{Data: append([]byte(nil), buf[:n]...)}); sendErr != nil {
				return sendErr
			}
		}

		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrNoProgress
		}
	}
}

func receiveVNCInput(stream grpc.BidiStreamingServer[machine.VNCRequest, machine.VNCResponse], conn io.Writer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		data, ok := req.GetRequest().(*machine.VNCRequest_Data)
		if !ok {
			return status.Error(codes.InvalidArgument, "expected VNC data")
		}

		if len(data.Data) > maxInputChunk {
			return status.Error(codes.ResourceExhausted, "VNC frame exceeds 64 KiB")
		}

		if err = writeInput(conn, data.Data); err != nil {
			return err
		}
	}
}
