// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

// Package vmconsole attaches a local terminal to a live VM serial stream.
package vmconsole

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/siderolabs/talos/internal/pkg/terminal"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

const inputChunkSize = 4096

var (
	errLocalDetach = errors.New("console local detach")
	errSendEOF     = errors.New("console send returned EOF")
)

type consoleStream = grpc.BidiStreamingClient[machine.ConsoleRequest, machine.ConsoleResponse]

// Run attaches to name and forwards raw bytes until Ctrl-], stdin EOF, cancellation,
// or a remote failure. It never sends a guest power or shutdown request. Input must
// be a pollable file descriptor with no other readers. Output must not block forever.
// Run joins its workers and restores terminal settings before returning, including
// on SIGTERM and SIGHUP. Ctrl-C is forwarded to the guest in raw mode.
func Run(ctx context.Context, client machine.HypervisorServiceClient, name string, input *os.File, output io.Writer) (result error) {
	if name == "" {
		return errors.New("console VM name must not be empty")
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := client.ConsoleStream(streamCtx)
	if err != nil {
		return fmt.Errorf("open console stream: %w", err)
	}

	if err = stream.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{Name: name},
		},
	}); err != nil {
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("attach console %q: %w", name, err)
		}

		err = forwardOutput(stream, output)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if errors.Is(err, io.EOF) {
			return nil
		}

		return fmt.Errorf("attach console %q: %w", name, err)
	}

	terminalState, err := terminal.MakeRaw(input)
	if err != nil {
		return fmt.Errorf("prepare console terminal: %w", err)
	}
	defer func() { result = errors.Join(result, terminalState.Restore()) }()

	inputDone := make(chan error, 1)
	go func() { inputDone <- forwardInput(streamCtx, stream, input) }()

	recvDone := make(chan error, 1)
	go func() { recvDone <- forwardOutput(stream, output) }()

	return awaitCompletion(ctx, cancel, inputDone, recvDone)
}

func awaitCompletion(ctx context.Context, cancel context.CancelFunc, inputDone, recvDone <-chan error) error {
	var err error

	select {
	case err = <-inputDone:
		if errors.Is(err, errSendEOF) {
			select {
			case err = <-recvDone:
			case <-ctx.Done():
				cancel()
				<-recvDone

				return ctx.Err()
			}

			break
		}

		cancel()
		<-recvDone
	case err = <-recvDone:
		cancel()
		<-inputDone
	case <-ctx.Done():
		cancel()
		<-inputDone
		<-recvDone
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}

func forwardOutput(stream consoleStream, output io.Writer) error {
	for {
		response, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("receive console output: %w", err)
		}

		data := response.GetStdoutData()

		if err := terminal.WriteOutput(output, data); err != nil {
			return err
		}
	}
}

func forwardInput(ctx context.Context, stream consoleStream, input *os.File) error {
	err := terminal.ReadInput(ctx, input, inputChunkSize, func(data []byte) error {
		detach, err := sendInput(stream, data)
		if err != nil {
			return err
		}

		if detach {
			return errLocalDetach
		}

		return nil
	})
	if errors.Is(err, errLocalDetach) {
		return nil
	}

	return err
}

func sendInput(stream consoleStream, data []byte) (bool, error) {
	// Ctrl-] is a local escape, never guest input.
	escape := bytes.IndexByte(data, 0x1d)
	if escape >= 0 {
		data = data[:escape]
	}

	if len(data) > 0 {
		if err := stream.Send(&machine.ConsoleRequest{
			Request: &machine.ConsoleRequest_StdinData{StdinData: data},
		}); err != nil {
			if errors.Is(err, io.EOF) {
				return false, fmt.Errorf("%w: %w", errSendEOF, err)
			}

			return false, fmt.Errorf("send console input: %w", err)
		}
	}

	return escape >= 0, nil
}
