// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vmvnc forwards a loopback viewer to a VM's authenticated VNC stream.
package vmvnc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

const chunkSize = 64 * 1024

var errSendEOF = errors.New("VNC send closed")

// Listen binds only IPv4 loopback. Port zero asks the OS for an available port.
func Listen(ctx context.Context, port int) (net.Listener, error) {
	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("invalid VNC TCP port %d", port)
	}

	return (&net.ListenConfig{}).Listen(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}

// Serve owns listener and closes it on return. It allows one active viewer and
// rejects additional connections without starting workers. Each viewer gets a
// fresh RPC; session errors are reported without stopping the listener. report
// must return promptly and is called by at most one worker at a time.
func Serve(ctx context.Context, client machine.HypervisorServiceClient, name string, listener net.Listener, report func(error)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer listener.Close() //nolint:errcheck

	if name == "" {
		return errors.New("VNC VM name must not be empty")
	}

	stop := context.AfterFunc(ctx, func() { listener.Close() }) //nolint:errcheck
	defer stop()

	var workers sync.WaitGroup
	defer workers.Wait()
	defer cancel()

	active := make(chan struct{}, 1)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf("accept VNC viewer: %w", err)
		}

		select {
		case active <- struct{}{}:
			workers.Go(func() {
				defer func() { <-active }()

				if err := relay(ctx, client, name, conn); err != nil && ctx.Err() == nil && report != nil {
					report(fmt.Errorf("VNC viewer session: %w", err))
				}
			})
		default:
			conn.Close() //nolint:errcheck
		}
	}
}

//nolint:gocyclo // Preserve remote errors while canceling and joining both relay directions.
func relay(ctx context.Context, client machine.HypervisorServiceClient, name string, conn net.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer conn.Close() //nolint:errcheck

	stream, err := client.VNCStream(ctx)
	if err != nil {
		return err
	}

	if err = stream.Send(&machine.VNCRequest{Request: &machine.VNCRequest_Attach{Attach: &machine.VNCAttach{Name: name}}}); err != nil {
		// Send can return EOF instead of the remote status. Receive that status.
		if errors.Is(err, io.EOF) {
			_, err = stream.Recv()
		}

		return err
	}

	inputDone := make(chan error, 1)
	outputDone := make(chan error, 1)

	go func() { inputDone <- forwardInput(conn, stream) }()
	go func() { outputDone <- forwardOutput(conn, stream) }()

	var inputFinished, outputFinished bool

	select {
	case err = <-inputDone:
		inputFinished = true

		// A failed gRPC Send exposes the actual remote status through Recv.
		// Do not cancel that receive and hide an authorization/setup error.
		if errors.Is(err, errSendEOF) {
			select {
			case err = <-outputDone:
				outputFinished = true
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
	case err = <-outputDone:
		outputFinished = true
	case <-ctx.Done():
		err = ctx.Err()
	}

	cancel()
	conn.Close() //nolint:errcheck

	if !inputFinished {
		<-inputDone
	}

	if !outputFinished {
		<-outputDone
	}

	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}

func forwardInput(conn net.Conn, stream machine.HypervisorService_VNCStreamClient) error {
	data := make([]byte, chunkSize)

	for {
		n, err := conn.Read(data)
		if n > 0 {
			// Do not mutate a message after handing it to gRPC.
			if sendErr := stream.Send(&machine.VNCRequest{Request: &machine.VNCRequest_Data{Data: append([]byte(nil), data[:n]...)}}); sendErr != nil {
				if errors.Is(sendErr, io.EOF) {
					return errSendEOF
				}

				return sendErr
			}
		}

		if err != nil {
			return err
		}
	}
}

func forwardOutput(conn net.Conn, stream machine.HypervisorService_VNCStreamClient) error {
	for {
		response, err := stream.Recv()
		if err != nil {
			return err
		}

		data := response.GetData()
		if len(data) > chunkSize {
			return errors.New("VNC response exceeds 64 KiB")
		}

		for len(data) > 0 {
			n, err := conn.Write(data)
			if err != nil {
				return err
			}

			if n == 0 {
				return io.ErrShortWrite
			}

			data = data[n:]
		}
	}
}
