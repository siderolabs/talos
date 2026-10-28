// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"syscall"

	"google.golang.org/grpc"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// containerStreamMaxMessageSize is the largest message of a container stream.
const containerStreamMaxMessageSize = 4 * 1024 * 1024

// TerminalSize is the size of a terminal, in characters.
type TerminalSize struct {
	Width  uint32
	Height uint32
}

// ContainerStreamOptions connects a command running on the node to the caller.
type ContainerStreamOptions struct {
	// Stdin is sent to the command until it returns EOF; nil sends no stdin at all.
	//
	// If it is an io.Closer, it is closed when the command ends, and Close must unblock a
	// concurrent Read (as io.Pipe does). A Read which never returns is otherwise left behind.
	Stdin io.Reader
	// Stdout receives the command's stdout, and its stderr where the API does not keep them apart.
	Stdout io.Writer
	// Stderr receives the command's stderr where the API keeps it apart.
	Stderr io.Writer
	// Resize sends terminal size changes to a command running with a terminal.
	Resize <-chan TerminalSize
	// Signal sends signals to the command.
	Signal <-chan syscall.Signal
}

// DebugContainerRun runs a debug container on the node and streams its stdio, returning its exit
// code.
func (c *Client) DebugContainerRun(ctx context.Context, spec *machineapi.DebugContainerRunRequestSpec, opts ContainerStreamOptions) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := c.DebugService().ContainerRun(
		ctx,
		grpc.MaxCallRecvMsgSize(containerStreamMaxMessageSize),
		grpc.MaxCallSendMsgSize(containerStreamMaxMessageSize),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create debug container stream: %w", err)
	}

	if err = stream.Send(&machineapi.DebugContainerRunRequest{
		Request: &machineapi.DebugContainerRunRequest_Spec{
			Spec: spec,
		},
	}); err != nil {
		return 0, fmt.Errorf("failed to send debug container spec: %w", err)
	}

	result, err := runContainerStream(cancel, debugRunStream{stream}, opts)

	return result.exitCode, err
}

// debugRunStream adapts ContainerRun to a container stream: stdin ends with a half-close, after
// which nothing else can be sent.
type debugRunStream struct {
	stream grpc.BidiStreamingClient[machineapi.DebugContainerRunRequest, machineapi.DebugContainerRunResponse]
}

func (d debugRunStream) sendStdin(data []byte) error {
	return d.stream.Send(&machineapi.DebugContainerRunRequest{
		Request: &machineapi.DebugContainerRunRequest_StdinData{StdinData: data},
	})
}

func (d debugRunStream) sendStdinEOF() (bool, error) {
	return true, d.stream.CloseSend()
}

func (d debugRunStream) sendSignal(signal syscall.Signal) error {
	return d.stream.Send(&machineapi.DebugContainerRunRequest{
		Request: &machineapi.DebugContainerRunRequest_Signal{Signal: int32(signal)},
	})
}

func (d debugRunStream) sendResize(size TerminalSize) error {
	return d.stream.Send(&machineapi.DebugContainerRunRequest{
		Request: &machineapi.DebugContainerRunRequest_TermResize{
			TermResize: &machineapi.DebugContainerTerminalResize{
				Width:  int32(size.Width),  //nolint:gosec
				Height: int32(size.Height), //nolint:gosec
			},
		},
	})
}

func (d debugRunStream) recv() (containerStreamEvent, error) {
	msg, err := d.stream.Recv()
	if err != nil {
		return containerStreamEvent{}, err
	}

	switch msg.Resp.(type) {
	case *machineapi.DebugContainerRunResponse_StdoutData:
		return containerStreamEvent{stdout: msg.GetStdoutData()}, nil
	case *machineapi.DebugContainerRunResponse_ExitCode:
		return containerStreamEvent{exited: true, exitCode: int(msg.GetExitCode())}, nil
	default:
		return containerStreamEvent{}, nil
	}
}

// ContainerExecResult is how a command run in a container ended.
type ContainerExecResult struct {
	// InstanceID is the container instance the command ran in.
	InstanceID string
	// ExitCode is the exit code of the command.
	ExitCode int
}

// ContainerExec runs a command in a running container on the node and streams its stdio.
//
// A command exiting with a nonzero code is a result, not an error. Canceling ctx ends the command.
// With a terminal, the end of stdin doesn't close the command's stdin, as that would hang up the terminal.
func (c *Client) ContainerExec(ctx context.Context, spec *machineapi.DebugContainerExecRequestSpec, opts ContainerStreamOptions) (ContainerExecResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := c.DebugService().ContainerExec(
		ctx,
		grpc.MaxCallRecvMsgSize(containerStreamMaxMessageSize),
		grpc.MaxCallSendMsgSize(containerStreamMaxMessageSize),
	)
	if err != nil {
		return ContainerExecResult{}, fmt.Errorf("failed to create container exec stream: %w", err)
	}

	if err = stream.Send(&machineapi.DebugContainerExecRequest{
		Request: &machineapi.DebugContainerExecRequest_Spec{
			Spec: spec,
		},
	}); err != nil {
		return ContainerExecResult{}, fmt.Errorf("failed to send container exec spec: %w", err)
	}

	result, err := runContainerStream(cancel, containerExecStream{stream}, opts)

	return ContainerExecResult{InstanceID: result.instanceID, ExitCode: result.exitCode}, err
}

// containerExecStream adapts ContainerExec to a container stream.
type containerExecStream struct {
	stream grpc.BidiStreamingClient[machineapi.DebugContainerExecRequest, machineapi.DebugContainerExecResponse]
}

func (e containerExecStream) sendStdin(data []byte) error {
	return e.stream.Send(&machineapi.DebugContainerExecRequest{
		Request: &machineapi.DebugContainerExecRequest_StdinData{StdinData: data},
	})
}

func (e containerExecStream) sendStdinEOF() (bool, error) {
	return false, e.stream.Send(&machineapi.DebugContainerExecRequest{
		Request: &machineapi.DebugContainerExecRequest_StdinEof{StdinEof: true},
	})
}

func (e containerExecStream) sendSignal(signal syscall.Signal) error {
	return e.stream.Send(&machineapi.DebugContainerExecRequest{
		Request: &machineapi.DebugContainerExecRequest_Signal{Signal: int32(signal)},
	})
}

func (e containerExecStream) sendResize(size TerminalSize) error {
	return e.stream.Send(&machineapi.DebugContainerExecRequest{
		Request: &machineapi.DebugContainerExecRequest_TermResize{
			TermResize: &machineapi.DebugContainerTerminalResize{
				Width:  int32(size.Width),  //nolint:gosec
				Height: int32(size.Height), //nolint:gosec
			},
		},
	})
}

func (e containerExecStream) recv() (containerStreamEvent, error) {
	msg, err := e.stream.Recv()
	if err != nil {
		return containerStreamEvent{}, err
	}

	switch msg.Resp.(type) {
	case *machineapi.DebugContainerExecResponse_InstanceId:
		return containerStreamEvent{instanceID: msg.GetInstanceId()}, nil
	case *machineapi.DebugContainerExecResponse_StdoutData:
		return containerStreamEvent{stdout: msg.GetStdoutData()}, nil
	case *machineapi.DebugContainerExecResponse_StderrData:
		return containerStreamEvent{stderr: msg.GetStderrData()}, nil
	case *machineapi.DebugContainerExecResponse_ExitCode:
		return containerStreamEvent{exited: true, exitCode: int(msg.GetExitCode())}, nil
	default:
		return containerStreamEvent{}, nil
	}
}

// containerStreamEvent is a message from the node, translated from the message type of the RPC.
type containerStreamEvent struct {
	stdout     []byte
	stderr     []byte
	instanceID string
	exited     bool
	exitCode   int
}

// containerStream is the client side of a debug RPC streaming the stdio of a command.
type containerStream interface {
	sendStdin(data []byte) error
	// sendStdinEOF reports whether the request stream is closed with it.
	sendStdinEOF() (bool, error)
	sendSignal(signal syscall.Signal) error
	sendResize(size TerminalSize) error
	recv() (containerStreamEvent, error)
}

// containerStreamResult is how a command streamed by a debug RPC ended.
type containerStreamResult struct {
	instanceID string
	exitCode   int
}

// containerStreamRequest is a message queued for sending.
type containerStreamRequest struct {
	stdin    []byte
	stdinEOF bool
	signal   syscall.Signal
	resize   *TerminalSize
}

// runContainerStream streams the stdio of a command until the node reports its exit code.
//
// A single goroutine sends on the stream. Once the exit code is received, cancel ends the RPC
// right away: older nodes wait for the client to hang up.
//
//nolint:gocyclo,cyclop
func runContainerStream(cancel context.CancelFunc, stream containerStream, opts ContainerStreamOptions) (containerStreamResult, error) {
	stdout, stderr := opts.Stdout, opts.Stderr
	if stdout == nil {
		stdout = io.Discard
	}

	if stderr == nil {
		stderr = io.Discard
	}

	producerCtx, stopProducers := context.WithCancel(context.Background())

	sendC := make(chan containerStreamRequest)
	senderDone := make(chan struct{})

	go func() {
		defer close(senderDone)

		closed := false

		for {
			var req containerStreamRequest

			select {
			case req = <-sendC:
			case <-producerCtx.Done():
				return
			}

			if closed {
				// A signal can't be forwarded once the request stream is closed, so the command
				// is ended instead: the node kills it when the RPC goes away.
				if req.signal != 0 {
					cancel()
				}

				continue
			}

			var err error

			switch {
			case req.stdinEOF:
				closed, err = stream.sendStdinEOF()
			case req.stdin != nil:
				err = stream.sendStdin(req.stdin)
			case req.signal != 0:
				err = stream.sendSignal(req.signal)
			case req.resize != nil:
				err = stream.sendResize(*req.resize)
			}

			// The receiving side reports why the stream failed.
			if err != nil {
				closed = true
			}
		}
	}()

	queue := func(req containerStreamRequest) bool {
		select {
		case sendC <- req:
			return true
		case <-producerCtx.Done():
			return false
		}
	}

	var producers sync.WaitGroup

	_, stdinClosable := opts.Stdin.(io.Closer)

	switch {
	case opts.Stdin == nil:
		producers.Go(func() { queue(containerStreamRequest{stdinEOF: true}) })
	case stdinClosable:
		producers.Go(func() { sendStdin(opts.Stdin, queue) })
	default:
		go sendStdin(opts.Stdin, queue)
	}

	if opts.Signal != nil {
		producers.Go(func() {
			for {
				select {
				case signal, ok := <-opts.Signal:
					if !ok || !queue(containerStreamRequest{signal: signal}) {
						return
					}
				case <-producerCtx.Done():
					return
				}
			}
		})
	}

	if opts.Resize != nil {
		producers.Go(func() {
			for {
				select {
				case size, ok := <-opts.Resize:
					if !ok || !queue(containerStreamRequest{resize: &size}) {
						return
					}
				case <-producerCtx.Done():
					return
				}
			}
		})
	}

	defer func() {
		cancel()
		stopProducers()

		if closer, ok := opts.Stdin.(io.Closer); ok {
			closer.Close() //nolint:errcheck
		}

		producers.Wait()
		<-senderDone
	}()

	var result containerStreamResult

	for {
		event, err := stream.recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return result, errors.New("stream ended without an exit code")
			}

			return result, err
		}

		if event.stdout != nil {
			if _, err = stdout.Write(event.stdout); err != nil {
				return result, fmt.Errorf("failed to write stdout: %w", err)
			}
		}

		if event.stderr != nil {
			if _, err = stderr.Write(event.stderr); err != nil {
				return result, fmt.Errorf("failed to write stderr: %w", err)
			}
		}

		if event.instanceID != "" {
			result.instanceID = event.instanceID
		}

		if event.exited {
			result.exitCode = event.exitCode

			return result, nil
		}
	}
}

// sendStdin queues stdin in chunks of its own, followed by stdin EOF.
func sendStdin(stdin io.Reader, queue func(containerStreamRequest) bool) {
	for {
		buf := make([]byte, 32*1024)

		n, err := stdin.Read(buf)
		if n > 0 && !queue(containerStreamRequest{stdin: buf[:n]}) {
			return
		}

		if err != nil {
			queue(containerStreamRequest{stdinEOF: true})

			return
		}
	}
}
