// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug

import (
	"context"
	"fmt"
	"io"
	"log"
	"syscall"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/siderolabs/gen/panicsafe"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func newGrpcStreamWriter(srv grpc.BidiStreamingServer[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse]) (
	*grpcStdioStreamer,
	io.Reader,
	io.Writer,
) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	return &grpcStdioStreamer{
		srv:     srv,
		stdinW:  stdinW,
		stdoutR: stdoutR,
		stdoutW: stdoutW,
	}, stdinR, stdoutW
}

type grpcStdioStreamer struct {
	srv grpc.BidiStreamingServer[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse]

	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
}

//nolint:gocyclo
func (g *grpcStdioStreamer) stream(ctx context.Context, statusC <-chan containerdapi.ExitStatus, task containerdapi.Task) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sendLoopCh, recvLoopCh := make(chan error), make(chan error)

	go func(errCh chan<- error) {
		errCh <- panicsafe.RunErr(g.sendLoop)
	}(sendLoopCh)

	go func(errCh chan<- error) {
		errCh <- panicsafe.RunErr(func() error {
			return g.recvLoop(ctx, task)
		})
	}(recvLoopCh)

	for {
		select {
		// task terminated
		case ec := <-statusC:
			// closing r.stdoutW causes the sendLoop, which s
			// blocking on r.stdoutR.Read(), to get an EOF and exit
			g.stdoutW.Close() //nolint:errcheck

			// close r.stdinW to ensure the container exits if it's still running
			g.stdinW.Close() //nolint:errcheck

			// cancel the context to stop loops
			cancel()

			if ec.Error() != nil {
				return ec.Error()
			}

			// wait for send loop to exit
			//
			// calling srv.Send from multiple goroutines is not safe,
			// so we need to wait for sendLoop to exit
			if sendLoopCh != nil {
				<-sendLoopCh
			}

			// then, sending the exit code back to the client makes
			// the client disconnect
			if err := g.srv.Send(&machine.DebugContainerRunResponse{
				Resp: &machine.DebugContainerRunResponse_ExitCode{
					ExitCode: int32(ec.ExitCode()),
				},
			}); err != nil {
				return fmt.Errorf("debug container: failed to send exit code: %w", err)
			}

			// wait for recv loop to exit after client disconnects
			if recvLoopCh != nil {
				<-recvLoopCh
			}

			return nil
		// our send loop terminated
		case sendErr := <-sendLoopCh:
			if sendErr == nil { // keep waiting for task to exit
				sendLoopCh = nil

				continue
			}

			// close r.stdinW to ensure the container exits if it's still running
			g.stdinW.Close() //nolint:errcheck

			return fmt.Errorf("debug container: send loop error: %w", sendErr)
		case recvErr := <-recvLoopCh:
			if recvErr == nil { // keep waiting for task to exit
				recvLoopCh = nil

				continue
			}

			// close r.stdoutW to stop the send loop
			g.stdoutW.Close() //nolint:errcheck

			return fmt.Errorf("debug container: receive loop error: %w", recvErr)
		// client walked away
		case <-ctx.Done():
			// closing r.stdoutW causes the sendLoop, which s
			// blocking on r.stdoutR.Read(), to get an EOF and exit
			g.stdoutW.Close() //nolint:errcheck

			// close r.stdinW to ensure the container exits if it's still running
			g.stdinW.Close() //nolint:errcheck

			// wait for loops to exit
			if recvLoopCh != nil {
				<-recvLoopCh
			}

			if sendLoopCh != nil {
				<-sendLoopCh
			}

			return ctx.Err()
		}
	}
}

func (g *grpcStdioStreamer) recvLoop(ctx context.Context, task containerdapi.Task) error {
	defer g.stdinW.Close() //nolint:errcheck

	for {
		msg, err := g.srv.Recv()
		if err != nil {
			if status.Code(err) != codes.Canceled && err != io.EOF {
				return fmt.Errorf("error receiving input message: %w", err)
			}

			return nil
		}

		if err = g.processMessage(ctx, task, msg); err != nil {
			return fmt.Errorf("error processing input message: %w", err)
		}
	}
}

func (g *grpcStdioStreamer) processMessage(ctx context.Context, task containerdapi.Task, msg *machine.DebugContainerRunRequest) error {
	switch msg.Request.(type) {
	case *machine.DebugContainerRunRequest_StdinData:
		if stdinData := msg.GetStdinData(); stdinData != nil {
			_, err := g.stdinW.Write(stdinData)
			if err != nil {
				return fmt.Errorf("failed to write to stdin: %w", err)
			}
		}

	case *machine.DebugContainerRunRequest_TermResize:
		if err := task.Resize(
			ctx,
			uint32(msg.GetTermResize().Width),
			uint32(msg.GetTermResize().Height),
		); err != nil {
			return fmt.Errorf("failed to resize terminal: %w", err)
		}

	case *machine.DebugContainerRunRequest_Signal:
		signalNum := msg.GetSignal()
		log.Printf("debug container: received signal %d, forwarding to task", signalNum)

		if err := task.Kill(ctx, syscall.Signal(signalNum)); err != nil {
			return fmt.Errorf("debug container: failed to forward signal to task: %w", err)
		}

	default:
		return fmt.Errorf("unknown request type: %T", msg.Request)
	}

	return nil
}

func (g *grpcStdioStreamer) sendLoop() error {
	defer g.stdoutW.Close() //nolint:errcheck

	b := make([]byte, 512)

	for {
		n, err := g.stdoutR.Read(b)
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return fmt.Errorf("failed to read from stdout: %w", err)
		}

		err = g.srv.Send(&machine.DebugContainerRunResponse{
			Resp: &machine.DebugContainerRunResponse_StdoutData{
				StdoutData: b[:n],
			},
		})
		if err != nil {
			if status.Code(err) != codes.Canceled {
				return fmt.Errorf("debug container: failed to send stdout data: %w", err)
			}

			return nil
		}
	}
}
