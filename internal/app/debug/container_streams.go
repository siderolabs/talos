// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"syscall"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	"github.com/siderolabs/gen/panicsafe"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// outputStream is the stream a chunk of process output was written to.
type outputStream int

const (
	outputStdout outputStream = iota
	outputStderr
)

// controlMessage is a request from the client, translated from the message type of the RPC.
type controlMessage struct {
	stdinData []byte
	stdinEOF  bool
	signal    int32
	resize    *machine.DebugContainerTerminalResize
}

// streamAdapter translates between the messages of a debug RPC and a session.
type streamAdapter interface {
	recv() (controlMessage, error)
	sendOutput(stream outputStream, data []byte) error
	sendExitCode(code int32) error
}

// processControl delivers signals and terminal resizes to the process of a session.
type processControl interface {
	signal(ctx context.Context, signal int32) error
	resize(ctx context.Context, width, height uint32) error
}

// exitResult is how the process of a session ended.
type exitResult struct {
	code int32
	err  error
}

// session streams the stdio of a single process over a debug RPC.
//
// A single goroutine sends on the stream, as sending from several goroutines is not safe. The
// exit code is sent last, once all the output of the process has been sent.
type session struct {
	adapter streamAdapter

	// pending counts output read from the process but not sent yet, and lastActivity is the time of
	// the last progress with the output.
	outputMu     sync.Mutex
	pending      int
	lastActivity time.Time

	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	stderrR *io.PipeReader
	stderrW *io.PipeWriter
}

// newSession creates a session; without separateStderr, stderr is written to stdout.
func newSession(adapter streamAdapter, separateStderr bool) *session {
	s := &session{adapter: adapter}

	s.stdinR, s.stdinW = io.Pipe()
	s.stdoutR, s.stdoutW = io.Pipe()

	if separateStderr {
		s.stderrR, s.stderrW = io.Pipe()
	}

	return s
}

// stdin is what the process reads its stdin from.
func (s *session) stdin() io.Reader { return s.stdinR }

// stdout is where the process writes its stdout to.
func (s *session) stdout() io.Writer { return s.stdoutW }

// stderr is where the process writes its stderr to.
func (s *session) stderr() io.Writer {
	if s.stderrW == nil {
		return s.stdoutW
	}

	return s.stderrW
}

func (s *session) closeOutput() {
	s.stdoutW.Close() //nolint:errcheck

	if s.stderrW != nil {
		s.stderrW.Close() //nolint:errcheck
	}
}

// close releases the stdio of a process the session did not run, or did not run to the end.
func (s *session) close() {
	s.closeOutput()
	s.closeStdin()
}

func (s *session) closeStdin() {
	s.stdinW.Close() //nolint:errcheck
	s.stdinR.Close() //nolint:errcheck
}

// run streams the process stdio until the process exits, the client goes away or the stream fails.
//
// drainOutput returns once the process output has been written to the session, or when its context
// is canceled.
//
//nolint:gocyclo,cyclop
func (s *session) run(ctx context.Context, control processControl, exitC <-chan exitResult, drainOutput func(context.Context)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sendC := make(chan func() error)
	sendErrC := make(chan error, 1)

	go func() {
		sendErrC <- panicsafe.RunErr(func() error {
			for {
				select {
				case send, ok := <-sendC:
					if !ok {
						return nil
					}

					if err := send(); err != nil {
						cancel()

						return err
					}
				case <-ctx.Done():
					return nil
				}
			}
		})
	}()

	var pumps sync.WaitGroup

	for stream, r := range map[outputStream]*io.PipeReader{outputStdout: s.stdoutR, outputStderr: s.stderrR} {
		if r == nil {
			continue
		}

		pumps.Go(func() {
			s.pump(ctx, stream, r, sendC)
		})
	}

	recvErrC := make(chan error, 1)

	go func() {
		recvErrC <- panicsafe.RunErr(func() error {
			return s.recvLoop(ctx, control)
		})
	}()

	finish := func(exitCode *int32) error {
		s.closeOutput()
		pumps.Wait()

		if exitCode != nil {
			code := *exitCode

			select {
			case sendC <- func() error { return s.adapter.sendExitCode(code) }:
			case <-ctx.Done():
			}
		}

		close(sendC)

		s.closeStdin()

		return sendError(<-sendErrC)
	}

	// The sender may be blocked on a client which stopped reading, and is only released once the
	// RPC returns, so an aborted session does not wait for it.
	abort := func(err error) error {
		cancel()
		s.closeOutput()
		s.closeStdin()

		return err
	}

	var finishC chan error

	for {
		select {
		case result := <-exitC:
			exitC = nil
			finishC = make(chan error, 1)

			go func() {
				s.waitOutput(ctx, drainOutput)

				if result.err != nil {
					finishC <- errors.Join(result.err, finish(nil))

					return
				}

				finishC <- finish(&result.code)
			}()
		case err := <-finishC:
			return err
		case err := <-recvErrC:
			if err == nil {
				recvErrC = nil

				continue
			}

			return abort(err)
		case <-ctx.Done():
			if finishC != nil {
				return <-finishC
			}

			if err := finish(nil); err != nil {
				return err
			}

			return ctx.Err()
		}
	}
}

// outputIdleTimeout ends the wait for more output of an exited process once none has come for that
// long.
const outputIdleTimeout = time.Second

// waitOutput waits for the output of the exited process to be drained.
//
// A process it left behind, e.g. in the background, can keep its output open, so once no output is
// pending any more the wait ends after outputIdleTimeout without more of it.
func (s *session) waitOutput(ctx context.Context, drainOutput func(context.Context)) {
	drainCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	drained := make(chan struct{})

	go func() {
		drainOutput(drainCtx)
		close(drained)
	}()

	idle := time.NewTimer(outputIdleTimeout)
	defer idle.Stop()

	for {
		select {
		case <-drained:
			return
		case <-ctx.Done():
			return
		case <-idle.C:
			s.outputMu.Lock()
			pending, idleFor := s.pending, time.Since(s.lastActivity)
			s.outputMu.Unlock()

			if pending == 0 && idleFor >= outputIdleTimeout {
				return
			}

			idle.Reset(max(outputIdleTimeout-idleFor, outputIdleTimeout/10))
		}
	}
}

func (s *session) outputProgress(pendingDelta int) {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()

	s.pending += pendingDelta
	s.lastActivity = time.Now()
}

// pump reads one output stream of the process and queues it for sending.
func (s *session) pump(ctx context.Context, stream outputStream, r *io.PipeReader, sendC chan<- func() error) {
	for {
		buf := make([]byte, 4096)

		n, err := r.Read(buf)
		if n > 0 {
			data := buf[:n]

			s.outputProgress(1)

			send := func() error {
				defer func() {
					s.outputProgress(-1)
				}()

				return s.adapter.sendOutput(stream, data)
			}

			select {
			case sendC <- send:
			case <-ctx.Done():
				r.CloseWithError(ctx.Err()) //nolint:errcheck

				return
			}
		}

		if err != nil {
			return
		}
	}
}

// recvLoop applies the client's requests until the client stops sending.
//
// A client which stops sending also ends the process stdin.
func (s *session) recvLoop(ctx context.Context, control processControl) error {
	defer s.stdinW.Close() //nolint:errcheck

	for {
		msg, err := s.adapter.recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				return nil
			}

			return fmt.Errorf("error receiving input message: %w", err)
		}

		if err = s.apply(ctx, control, msg); err != nil {
			return err
		}
	}
}

// apply applies a single request of the client.
func (s *session) apply(ctx context.Context, control processControl, msg controlMessage) error {
	switch {
	case msg.stdinData != nil:
		if _, err := s.stdinW.Write(msg.stdinData); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			return fmt.Errorf("failed to write to stdin: %w", err)
		}
	case msg.stdinEOF:
		s.stdinW.Close() //nolint:errcheck
	case msg.signal != 0:
		if err := control.signal(ctx, msg.signal); err != nil {
			return fmt.Errorf("failed to forward signal: %w", err)
		}
	case msg.resize != nil:
		if err := control.resize(ctx, uint32(msg.resize.Width), uint32(msg.resize.Height)); err != nil { //nolint:gosec
			return fmt.Errorf("failed to resize terminal: %w", err)
		}
	}

	return nil
}

// sendError reports a failure to send, unless the client has gone away.
func sendError(err error) error {
	if err == nil || status.Code(err) == codes.Canceled {
		return nil
	}

	return fmt.Errorf("failed to send: %w", err)
}

// containerdProcess controls the containerd process of a session.
type containerdProcess struct {
	process containerdapi.Process
}

// signal forwards a signal to the process; one which has exited already is no longer there to get it.
func (p containerdProcess) signal(ctx context.Context, signal int32) error {
	log.Printf("debug: forwarding signal %d to process %s", signal, p.process.ID())

	if err := p.process.Kill(ctx, syscall.Signal(signal)); err != nil && !errdefs.IsNotFound(err) {
		return err
	}

	return nil
}

func (p containerdProcess) resize(ctx context.Context, width, height uint32) error {
	if err := p.process.Resize(ctx, width, height); err != nil && !errdefs.IsNotFound(err) {
		return err
	}

	return nil
}

// processExit reports the exit of a containerd process.
func processExit(statusC <-chan containerdapi.ExitStatus) <-chan exitResult {
	exitC := make(chan exitResult, 1)

	go func() {
		status := <-statusC

		exitC <- exitResult{
			code: int32(status.ExitCode()), //nolint:gosec
			err:  status.Error(),
		}
	}()

	return exitC
}

// ioCreator creates the stdio of a containerd process connected to the session.
//
// Canceling it also releases the session's side: containerd waits for the stdio to be copied once
// it cancels it, e.g. when starting the process fails, which a write blocked on the session would
// never let happen.
func (s *session) ioCreator(opts ...cio.Opt) cio.Creator {
	creator := cio.NewCreator(opts...)

	return func(id string) (cio.IO, error) {
		processIO, err := creator(id)
		if err != nil {
			return nil, err
		}

		return sessionIO{IO: processIO, session: s}, nil
	}
}

// sessionIO is the stdio of a containerd process connected to a session.
type sessionIO struct {
	cio.IO

	session *session
}

func (i sessionIO) Cancel() {
	i.session.close()
	i.IO.Cancel()
}

// processOutputDrained returns once containerd has copied all output of the process.
func processOutputDrained(process containerdapi.Process) func(context.Context) {
	return func(ctx context.Context) {
		done := make(chan struct{})

		go func() {
			process.IO().Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
		}
	}
}
