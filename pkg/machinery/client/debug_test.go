// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client //nolint:testpackage // to test the unexported session with a fake stream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// fakeContainerStream records what the session sends, and replays events to it.
type fakeContainerStream struct {
	closesOnStdinEOF bool

	events chan containerStreamEvent
	errC   chan error

	mu       sync.Mutex
	stdin    bytes.Buffer
	stdinEOF bool
	signals  []syscall.Signal
	resizes  []TerminalSize
	afterEOF int
}

func newFakeContainerStream(closesOnStdinEOF bool) *fakeContainerStream {
	return &fakeContainerStream{
		closesOnStdinEOF: closesOnStdinEOF,
		events:           make(chan containerStreamEvent),
		errC:             make(chan error, 1),
	}
}

func (f *fakeContainerStream) sendStdin(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stdin.Write(data)

	return nil
}

func (f *fakeContainerStream) sendStdinEOF() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stdinEOF = true

	return f.closesOnStdinEOF, nil
}

func (f *fakeContainerStream) sendSignal(signal syscall.Signal) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.stdinEOF {
		f.afterEOF++
	}

	f.signals = append(f.signals, signal)

	return nil
}

func (f *fakeContainerStream) sendResize(size TerminalSize) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.resizes = append(f.resizes, size)

	return nil
}

func (f *fakeContainerStream) recv() (containerStreamEvent, error) {
	select {
	case event := <-f.events:
		return event, nil
	case err := <-f.errC:
		return containerStreamEvent{}, err
	}
}

func (f *fakeContainerStream) state() (string, bool, []syscall.Signal) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.stdin.String(), f.stdinEOF, append([]syscall.Signal(nil), f.signals...)
}

func runFakeContainerStream(t *testing.T, stream *fakeContainerStream, opts ContainerStreamOptions) <-chan error {
	t.Helper()

	errC := make(chan error, 1)

	go func() {
		_, err := runContainerStream(func() {}, stream, opts)
		errC <- err
	}()

	return errC
}

func TestContainerStreamOutputAndExitCode(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(false)

	var stdout, stderr bytes.Buffer

	resultC := make(chan containerStreamResult, 1)

	go func() {
		result, err := runContainerStream(func() {}, stream, ContainerStreamOptions{Stdout: &stdout, Stderr: &stderr})
		assert.NoError(t, err)

		resultC <- result
	}()

	stream.events <- containerStreamEvent{instanceID: "app-3"}

	stream.events <- containerStreamEvent{stdout: []byte("out")}

	stream.events <- containerStreamEvent{stderr: []byte("err")}

	stream.events <- containerStreamEvent{exited: true, exitCode: 7}

	result := <-resultC

	assert.Equal(t, containerStreamResult{instanceID: "app-3", exitCode: 7}, result)
	assert.Equal(t, "out", stdout.String())
	assert.Equal(t, "err", stderr.String())
}

func TestContainerStreamStdin(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(false)

	// Read in small chunks: every chunk must be sent as it was read, even though the session
	// reads ahead of the sender.
	data := strings.Repeat("0123456789", 100000)

	errC := runFakeContainerStream(t, stream, ContainerStreamOptions{Stdin: io.NopCloser(&smallReader{r: strings.NewReader(data)})})

	require.Eventually(t, func() bool {
		_, eof, _ := stream.state()

		return eof
	}, 10*time.Second, 10*time.Millisecond)

	stdin, _, _ := stream.state()
	assert.Equal(t, data, stdin)

	stream.events <- containerStreamEvent{exited: true}

	require.NoError(t, <-errC)
}

// TestContainerStreamSignalsAfterStdinEOF covers a command still running after its stdin ended: it
// keeps getting signals, unless the API ends the request stream with stdin.
func TestContainerStreamSignalsAfterStdinEOF(t *testing.T) {
	t.Parallel()

	for _, closesOnStdinEOF := range []bool{false, true} {
		t.Run(map[bool]string{false: "kept open", true: "closed"}[closesOnStdinEOF], func(t *testing.T) {
			t.Parallel()

			stream := newFakeContainerStream(closesOnStdinEOF)
			signalC := make(chan syscall.Signal)

			errC := runFakeContainerStream(t, stream, ContainerStreamOptions{Signal: signalC})

			require.Eventually(t, func() bool {
				_, eof, _ := stream.state()

				return eof
			}, 10*time.Second, 10*time.Millisecond)

			signalC <- syscall.SIGTERM

			if closesOnStdinEOF {
				// Nothing observable to wait for: give the sender a moment.
				time.Sleep(100 * time.Millisecond)
			} else {
				require.Eventually(t, func() bool {
					_, _, signals := stream.state()

					return len(signals) == 1
				}, 10*time.Second, 10*time.Millisecond)
			}

			stream.events <- containerStreamEvent{exited: true}

			require.NoError(t, <-errC)

			_, _, signals := stream.state()

			if closesOnStdinEOF {
				assert.Empty(t, signals)
			} else {
				assert.Equal(t, []syscall.Signal{syscall.SIGTERM}, signals)
			}
		})
	}
}

// TestContainerStreamBlockedStdin covers a command which exits while stdin is still open.
func TestContainerStreamBlockedStdin(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(false)
	stdinR, stdinW := io.Pipe()

	t.Cleanup(func() { stdinW.Close() }) //nolint:errcheck

	errC := runFakeContainerStream(t, stream, ContainerStreamOptions{Stdin: stdinR})

	stream.events <- containerStreamEvent{exited: true, exitCode: 1}

	select {
	case err := <-errC:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("session did not end while stdin was blocked")
	}
}

func TestContainerStreamWithoutExitCode(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		err         error
		expectedErr string
	}{
		{
			name:        "end of stream",
			err:         io.EOF,
			expectedErr: "stream ended without an exit code",
		},
		{
			name:        "error",
			err:         errors.New("connection lost"),
			expectedErr: "connection lost",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stream := newFakeContainerStream(false)
			stream.errC <- test.err

			_, err := runContainerStream(func() {}, stream, ContainerStreamOptions{})
			require.EqualError(t, err, test.expectedErr)
		})
	}
}

func TestContainerStreamWriteFailure(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(false)

	canceled := make(chan struct{})

	errC := make(chan error, 1)

	go func() {
		_, err := runContainerStream(func() { close(canceled) }, stream, ContainerStreamOptions{Stdout: failingWriter{}})
		errC <- err
	}()

	stream.events <- containerStreamEvent{stdout: []byte("out")}

	require.ErrorContains(t, <-errC, "failed to write stdout")

	select {
	case <-canceled:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream was not canceled")
	}
}

func TestContainerStreamContextCanceled(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(false)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	stream.errC <- ctx.Err()

	_, err := runContainerStream(func() {}, stream, ContainerStreamOptions{})
	require.ErrorIs(t, err, context.Canceled)
}

type smallReader struct {
	r io.Reader
}

func (s *smallReader) Read(p []byte) (int, error) {
	if len(p) > 7 {
		p = p[:7]
	}

	return s.r.Read(p)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

// TestContainerStreamHangsUpAfterExitCode covers older nodes, which keep the stream open after the
// exit code until the client hangs up.
func TestContainerStreamHangsUpAfterExitCode(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(false)

	hungUp := make(chan struct{})
	errC := make(chan error, 1)

	go func() {
		_, err := runContainerStream(func() { close(hungUp) }, stream, ContainerStreamOptions{})
		errC <- err
	}()

	stream.events <- containerStreamEvent{exited: true}

	require.NoError(t, <-errC)

	select {
	case <-hungUp:
	default:
		t.Fatal("the stream was not canceled after the exit code")
	}
}

// TestContainerStreamClosedControls covers callers closing their signal and resize channels.
func TestContainerStreamClosedControls(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(false)

	signalC := make(chan syscall.Signal)
	resizeC := make(chan TerminalSize)

	close(signalC)
	close(resizeC)

	errC := runFakeContainerStream(t, stream, ContainerStreamOptions{Signal: signalC, Resize: resizeC})

	time.Sleep(100 * time.Millisecond)

	stream.events <- containerStreamEvent{exited: true}

	require.NoError(t, <-errC)

	stream.mu.Lock()
	defer stream.mu.Unlock()

	assert.Empty(t, stream.signals)
	assert.Empty(t, stream.resizes)
}

// TestContainerStreamSignalAfterHalfClose covers a signal which can't be forwarded any more as
// stdin ended the request stream: the command is ended instead.
func TestContainerStreamSignalAfterHalfClose(t *testing.T) {
	t.Parallel()

	stream := newFakeContainerStream(true)
	signalC := make(chan syscall.Signal)

	canceled := make(chan struct{})

	var once sync.Once

	errC := make(chan error, 1)

	go func() {
		_, err := runContainerStream(func() { once.Do(func() { close(canceled) }) }, stream, ContainerStreamOptions{Signal: signalC})
		errC <- err
	}()

	require.Eventually(t, func() bool {
		_, eof, _ := stream.state()

		return eof
	}, 10*time.Second, 10*time.Millisecond)

	signalC <- syscall.SIGINT

	select {
	case <-canceled:
	case <-time.After(10 * time.Second):
		t.Fatal("the command was not ended")
	}

	stream.errC <- context.Canceled

	require.ErrorIs(t, <-errC, context.Canceled)
}

// fakeExecClientStream stands in for the client side of a ContainerExec stream.
type fakeExecClientStream struct {
	grpc.ClientStream

	sent      []*machineapi.DebugContainerExecRequest
	responses []*machineapi.DebugContainerExecResponse
}

func (f *fakeExecClientStream) Send(req *machineapi.DebugContainerExecRequest) error {
	f.sent = append(f.sent, req)

	return nil
}

func (f *fakeExecClientStream) Recv() (*machineapi.DebugContainerExecResponse, error) {
	if len(f.responses) == 0 {
		return nil, io.EOF
	}

	resp := f.responses[0]
	f.responses = f.responses[1:]

	return resp, nil
}

// TestContainerExecStream covers the translation between ContainerExec messages and the session.
func TestContainerExecStream(t *testing.T) {
	t.Parallel()

	fake := &fakeExecClientStream{
		responses: []*machineapi.DebugContainerExecResponse{
			{Resp: &machineapi.DebugContainerExecResponse_InstanceId{InstanceId: "app-3"}},
			{Resp: &machineapi.DebugContainerExecResponse_StdoutData{StdoutData: []byte("out")}},
			{Resp: &machineapi.DebugContainerExecResponse_StderrData{StderrData: []byte("err")}},
			{Resp: &machineapi.DebugContainerExecResponse_ExitCode{ExitCode: 3}},
		},
	}

	var stdout, stderr bytes.Buffer

	result, err := runContainerStream(func() {}, containerExecStream{fake}, ContainerStreamOptions{Stdout: &stdout, Stderr: &stderr})
	require.NoError(t, err)

	assert.Equal(t, containerStreamResult{instanceID: "app-3", exitCode: 3}, result)
	assert.Equal(t, "out", stdout.String())
	assert.Equal(t, "err", stderr.String())

	// stdin ends with a message of its own, so the request stream stays open for signals.
	closed, err := containerExecStream{fake}.sendStdinEOF()
	require.NoError(t, err)
	assert.False(t, closed)
	assert.True(t, fake.sent[len(fake.sent)-1].GetStdinEof())
}
