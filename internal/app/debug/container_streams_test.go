// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug //nolint:testpackage // to test the unexported session with a fake stream and process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// fakeStream records what a session sends, in order, and feeds it control messages.
type fakeStream struct {
	requests chan controlMessage

	mu       sync.Mutex
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	exitCode *int32
	afterEOF bool
	sendErr  error
}

func newFakeStream() *fakeStream {
	return &fakeStream{requests: make(chan controlMessage)}
}

func (f *fakeStream) recv() (controlMessage, error) {
	msg, ok := <-f.requests
	if !ok {
		return controlMessage{}, io.EOF
	}

	return msg, nil
}

func (f *fakeStream) sendOutput(stream outputStream, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.sendErr != nil {
		return f.sendErr
	}

	if f.exitCode != nil {
		f.afterEOF = true
	}

	if stream == outputStderr {
		f.stderr.Write(data)
	} else {
		f.stdout.Write(data)
	}

	return nil
}

func (f *fakeStream) sendExitCode(code int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.exitCode = &code

	return nil
}

// fakeProcess records the signals and resizes a session delivers.
type fakeProcess struct {
	mu      sync.Mutex
	signals []int32
	resizes [][2]uint32
}

func (p *fakeProcess) signal(_ context.Context, signal int32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.signals = append(p.signals, signal)

	return nil
}

func (p *fakeProcess) resize(_ context.Context, width, height uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.resizes = append(p.resizes, [2]uint32{width, height})

	return nil
}

// TestSessionDeliversAllOutputBeforeExit covers a process which exits while its output is still
// being copied, as containerd reports the exit independently of copying the output.
func TestSessionDeliversAllOutputBeforeExit(t *testing.T) {
	t.Parallel()

	for _, separateStderr := range []bool{false, true} {
		t.Run(map[bool]string{false: "merged", true: "separate"}[separateStderr], func(t *testing.T) {
			t.Parallel()

			stream := newFakeStream()

			t.Cleanup(func() { close(stream.requests) })

			sess := newSession(stream, separateStderr)

			stdoutData := strings.Repeat("o", 1<<20)
			stderrData := strings.Repeat("e", 1<<20)

			var copying sync.WaitGroup

			copying.Go(func() { io.WriteString(sess.stdout(), stdoutData) }) //nolint:errcheck
			copying.Go(func() { io.WriteString(sess.stderr(), stderrData) }) //nolint:errcheck

			exitC := make(chan exitResult, 1)
			exitC <- exitResult{code: 3}

			require.NoError(t, sess.run(t.Context(), &fakeProcess{}, exitC, func(context.Context) { copying.Wait() }))

			stream.mu.Lock()
			defer stream.mu.Unlock()

			require.NotNil(t, stream.exitCode)
			assert.EqualValues(t, 3, *stream.exitCode)
			assert.False(t, stream.afterEOF, "output was sent after the exit code")

			if separateStderr {
				assert.Equal(t, stdoutData, stream.stdout.String())
				assert.Equal(t, stderrData, stream.stderr.String())
			} else {
				assert.Equal(t, len(stdoutData)+len(stderrData), stream.stdout.Len())
				assert.Zero(t, stream.stderr.Len())
			}
		})
	}
}

// TestSessionOutputHeldOpen covers a process which exited, leaving a process behind which keeps its
// output open: the session still ends with the exit code, after the output which was written.
func TestSessionOutputHeldOpen(t *testing.T) {
	t.Parallel()

	stream := newFakeStream()

	t.Cleanup(func() { close(stream.requests) })

	sess := newSession(stream, false)

	go io.WriteString(sess.stdout(), "started\n") //nolint:errcheck

	exitC := make(chan exitResult, 1)
	exitC <- exitResult{code: 0}

	errC := make(chan error, 1)

	go func() {
		// The output is never drained: the process left behind holds it open.
		errC <- sess.run(t.Context(), &fakeProcess{}, exitC, func(ctx context.Context) { <-ctx.Done() })
	}()

	select {
	case err := <-errC:
		require.NoError(t, err)
	case <-time.After(outputIdleTimeout + 10*time.Second):
		t.Fatal("the session waited for output held open by a process left behind")
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()

	require.NotNil(t, stream.exitCode)
	assert.Equal(t, "started\n", stream.stdout.String())
}

// TestSessionSlowClientNotCutOff covers output which takes longer than outputIdleTimeout to send:
// output already read is pending, so it is not given up on.
func TestSessionSlowClientNotCutOff(t *testing.T) {
	t.Parallel()

	stream := &slowStream{fakeStream: newFakeStream(), delay: outputIdleTimeout * 3 / 2}

	t.Cleanup(func() { close(stream.requests) })

	sess := newSession(stream, false)

	const chunks = 5

	go func() {
		for i := range chunks {
			fmt.Fprintf(sess.stdout(), "chunk %d\n", i) //nolint:errcheck
		}
	}()

	exitC := make(chan exitResult, 1)
	exitC <- exitResult{code: 0}

	require.NoError(t, sess.run(t.Context(), &fakeProcess{}, exitC, func(ctx context.Context) { <-ctx.Done() }))

	stream.mu.Lock()
	defer stream.mu.Unlock()

	require.NotNil(t, stream.exitCode)
	assert.False(t, stream.afterEOF, "output was sent after the exit code")
	assert.Equal(t, chunks, strings.Count(stream.stdout.String(), "chunk"), "got %q", stream.stdout.String())
}

// slowStream sends output slowly, as a client reading slowly would make it.
type slowStream struct {
	*fakeStream

	delay time.Duration
}

func (s *slowStream) sendOutput(stream outputStream, data []byte) error {
	time.Sleep(s.delay)

	return s.fakeStream.sendOutput(stream, data)
}

func TestSessionStdin(t *testing.T) {
	t.Parallel()

	stream := newFakeStream()
	sess := newSession(stream, true)

	stdinC := make(chan string, 1)

	go func() {
		data, _ := io.ReadAll(sess.stdin()) //nolint:errcheck
		stdinC <- string(data)
	}()

	exitC := make(chan exitResult, 1)
	process := &fakeProcess{}

	errC := make(chan error, 1)

	go func() {
		errC <- sess.run(t.Context(), process, exitC, func(context.Context) {})
	}()

	stream.requests <- controlMessage{stdinData: []byte("first ")}

	stream.requests <- controlMessage{stdinData: []byte("second")}

	stream.requests <- controlMessage{stdinEOF: true}

	assert.Equal(t, "first second", <-stdinC)

	// Signals and resizes still reach the process once stdin is closed.
	stream.requests <- controlMessage{signal: 15}

	stream.requests <- controlMessage{resize: &machine.DebugContainerTerminalResize{Width: 80, Height: 24}}

	close(stream.requests)

	exitC <- exitResult{code: 0}

	require.NoError(t, <-errC)

	process.mu.Lock()
	defer process.mu.Unlock()

	assert.Equal(t, []int32{15}, process.signals)
	assert.Equal(t, [][2]uint32{{80, 24}}, process.resizes)
}

// TestSessionHalfCloseEndsStdin covers clients which end stdin by closing their side of the stream.
func TestSessionHalfCloseEndsStdin(t *testing.T) {
	t.Parallel()

	stream := newFakeStream()
	sess := newSession(stream, false)

	stdinC := make(chan string, 1)

	go func() {
		data, _ := io.ReadAll(sess.stdin()) //nolint:errcheck
		stdinC <- string(data)
	}()

	exitC := make(chan exitResult, 1)
	errC := make(chan error, 1)

	go func() {
		errC <- sess.run(t.Context(), &fakeProcess{}, exitC, func(context.Context) {})
	}()

	stream.requests <- controlMessage{stdinData: []byte("data")}

	close(stream.requests)

	assert.Equal(t, "data", <-stdinC)

	exitC <- exitResult{code: 0}

	require.NoError(t, <-errC)
}

func TestSessionClientGone(t *testing.T) {
	t.Parallel()

	stream := newFakeStream()

	t.Cleanup(func() { close(stream.requests) })

	sess := newSession(stream, true)

	ctx, cancel := context.WithCancel(t.Context())

	errC := make(chan error, 1)

	go func() {
		errC <- sess.run(ctx, &fakeProcess{}, make(chan exitResult), func(context.Context) {})
	}()

	// The process keeps writing; the session must not hang on it once the client is gone.
	go io.Copy(sess.stdout(), infiniteReader{}) //nolint:errcheck

	cancel()

	select {
	case err := <-errC:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("session did not end when the client went away")
	}

	assert.Nil(t, stream.exitCode)
}

func TestSessionSendError(t *testing.T) {
	t.Parallel()

	stream := newFakeStream()

	t.Cleanup(func() { close(stream.requests) })

	stream.sendErr = errors.New("broken stream")

	sess := newSession(stream, true)

	errC := make(chan error, 1)

	go func() {
		errC <- sess.run(t.Context(), &fakeProcess{}, make(chan exitResult), func(context.Context) {})
	}()

	go io.Copy(sess.stdout(), infiniteReader{}) //nolint:errcheck

	select {
	case err := <-errC:
		require.ErrorContains(t, err, "broken stream")
	case <-time.After(10 * time.Second):
		t.Fatal("session did not end on a send error")
	}
}

// TestSessionReceiveErrorWithBlockedSend covers a client which stops reading output and then
// sends an invalid request: the session must not wait for the blocked send to return.
func TestSessionReceiveErrorWithBlockedSend(t *testing.T) {
	t.Parallel()

	stream := &blockingStream{
		fakeStream: newFakeStream(),
		sending:    make(chan struct{}, 1),
		release:    make(chan struct{}),
		recvErr:    make(chan error, 1),
	}

	t.Cleanup(func() { close(stream.release) })

	sess := newSession(stream, false)

	errC := make(chan error, 1)

	go func() {
		errC <- sess.run(t.Context(), &fakeProcess{}, make(chan exitResult), func(context.Context) {})
	}()

	go io.WriteString(sess.stdout(), "output") //nolint:errcheck

	<-stream.sending

	stream.recvErr <- errors.New("invalid request")

	select {
	case err := <-errC:
		require.ErrorContains(t, err, "invalid request")
	case <-time.After(10 * time.Second):
		t.Fatal("session waited for a blocked send")
	}
}

// TestSessionReceiveErrorWhileDraining is the same while the output of an exited process is drained.
func TestSessionReceiveErrorWhileDraining(t *testing.T) {
	t.Parallel()

	stream := &blockingStream{
		fakeStream: newFakeStream(),
		sending:    make(chan struct{}, 1),
		release:    make(chan struct{}),
		recvErr:    make(chan error, 1),
	}

	t.Cleanup(func() { close(stream.release) })

	sess := newSession(stream, false)

	exitC := make(chan exitResult, 1)
	errC := make(chan error, 1)

	go func() {
		errC <- sess.run(t.Context(), &fakeProcess{}, exitC, func(ctx context.Context) { <-ctx.Done() })
	}()

	go io.WriteString(sess.stdout(), "output") //nolint:errcheck

	<-stream.sending

	exitC <- exitResult{code: 0}

	stream.recvErr <- errors.New("invalid request")

	select {
	case err := <-errC:
		require.ErrorContains(t, err, "invalid request")
	case <-time.After(10 * time.Second):
		t.Fatal("session waited for a blocked send while draining")
	}
}

// blockingStream blocks every send until released, and fails to receive on request.
type blockingStream struct {
	*fakeStream

	sending chan struct{}
	release chan struct{}
	recvErr chan error
}

func (b *blockingStream) recv() (controlMessage, error) {
	return controlMessage{}, <-b.recvErr
}

func (b *blockingStream) sendOutput(outputStream, []byte) error {
	select {
	case b.sending <- struct{}{}:
	default:
	}

	<-b.release

	return errors.New("stream closed")
}

type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}

	return len(p), nil
}
