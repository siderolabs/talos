// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"io"
	"log"
	"syscall"

	containerdapi "github.com/containerd/containerd/v2/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func newGrpcStreamWriter(srv machine.MachineService_DebugContainerRunServer) (
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
	srv machine.MachineService_DebugContainerRunServer

	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
}

func (g *grpcStdioStreamer) stream(statusC <-chan containerdapi.ExitStatus, task containerdapi.Task) {
	recvLoopC := make(chan struct{})

	go func() {
		g.recvLoop(task)

		recvLoopC <- struct{}{}
	}()

	sendLoopC := make(chan struct{})

	go func() {
		g.sendLoop()

		sendLoopC <- struct{}{}
	}()

	select {
	case ec := <-statusC:
		// closing r.stdoutW causes the sendLoop, which s
		// blocking on r.stdoutR.Read(), to get an EOF and exit
		g.stdoutW.Close() //nolint:errcheck
		<-sendLoopC

		// then, sending the exit code back to the client makes
		// the client disconnect, causing the recvLoop which is
		// hanging on srv.Recv() to exit
		if err := g.srv.Send(&machine.DebugContainerRunResponse{
			Resp: &machine.DebugContainerRunResponse_ExitCode{
				ExitCode: int32(ec.ExitCode()),
			},
		}); err != nil {
			log.Printf("debug container: failed to send exit code: %s", err.Error())
		}

		<-recvLoopC

		return

	case <-recvLoopC:
	}

	// the client has disconnected, so we close r.stdinW
	// causing the container to exit
	g.stdinW.Close() //nolint:errcheck
	<-statusC

	// close stdoutW so that sendLoop will get an EOF
	// and exit
	g.stdoutW.Close() //nolint:errcheck
	<-sendLoopC
}

func (g *grpcStdioStreamer) recvLoop(task containerdapi.Task) {
	for {
		msg, err := g.srv.Recv()
		if err != nil {
			if status.Code(err) != codes.Canceled {
				log.Printf("debug container: recv error: %s", err.Error())
			}

			break
		}

		g.processMessage(task, msg)
	}
}

func (g *grpcStdioStreamer) processMessage(task containerdapi.Task, msg *machine.DebugContainerRunRequest) {
	switch msg.Request.(type) {
	case *machine.DebugContainerRunRequest_StdinData:
		if stdinData := msg.GetStdinData(); stdinData != nil {
			_, err := g.stdinW.Write(stdinData)
			if err != nil {
				if err != io.EOF {
					log.Printf("debug container: failed to write stdin: %s", err.Error())
				}

				break
			}
		}

	case *machine.DebugContainerRunRequest_TermResize:
		err := task.Resize(
			context.Background(),
			uint32(msg.GetTermResize().Width),
			uint32(msg.GetTermResize().Height))
		if err != nil {
			log.Printf("debug container: failed to resize terminal: %v", err)
		}

	case *machine.DebugContainerRunRequest_Signal:
		signalNum := msg.GetSignal()
		log.Printf("debug container: received signal %d, forwarding to task", signalNum)

		if err := task.Kill(context.Background(), syscall.Signal(signalNum)); err != nil {
			log.Printf("debug container: failed to forward signal to task: %v", err)
		}

	default:
		log.Printf("debug container: unknown request type")
	}
}

func (g *grpcStdioStreamer) sendLoop() {
	b := make([]byte, 512)

	for {
		n, err := g.stdoutR.Read(b)
		if err != nil {
			if err != io.EOF {
				log.Printf("debug container: failed to read stdout: %s", err.Error())
			}

			break
		}

		err = g.srv.Send(&machine.DebugContainerRunResponse{
			Resp: &machine.DebugContainerRunResponse_StdoutData{
				StdoutData: b[:n],
			},
		})
		if err != nil {
			if status.Code(err) != codes.Canceled {
				log.Printf("debug container: failed to send stdout data: %s", err.Error())
			}

			break
		}
	}
}
