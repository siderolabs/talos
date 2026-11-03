// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug //nolint:testpackage // to test the unexported pty helpers

import (
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestPty(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	master, slave, err := openPty()
	if err != nil {
		t.Skipf("no pty available: %v", err)
	}

	t.Cleanup(func() {
		master.Close() //nolint:errcheck
		slave.Close()  //nolint:errcheck
	})

	return master, slave
}

// TestWaitPtyDrainedSlowConsumer covers the last output of an exited child reaching a client which
// reads slower than the drain timeout: output still in the pty is not cut off.
func TestWaitPtyDrainedSlowConsumer(t *testing.T) {
	t.Parallel()

	master, slave := openTestPty(t)

	// More than one read from the pty master, so the relay is blocked on a slow write while more
	// output is waiting.
	output := strings.Repeat("0123456789abcdef", 1024)

	r, w := io.Pipe()

	var childExited atomic.Bool

	relayDone := make(chan struct{})
	waitDone := make(chan struct{})

	go func() {
		defer close(waitDone)

		waitPtyDrained(t.Context(), master, &childExited, relayDone)
	}()

	require.Eventually(t, childExited.Load, time.Second, time.Millisecond)

	go func() {
		slave.WriteString(output) //nolint:errcheck
	}()

	go func() {
		defer close(relayDone)

		relayPty(master, w, &childExited)
	}()

	receivedC := make(chan string, 1)

	go func() {
		time.Sleep(ptyDrainTimeout + time.Second)

		data, _ := io.ReadAll(r) //nolint:errcheck
		receivedC <- string(data)
	}()

	<-waitDone

	// As the session does once the child is done: whatever was not written by now is lost.
	w.CloseWithError(io.ErrClosedPipe) //nolint:errcheck

	received := strings.ReplaceAll(<-receivedC, "\r", "")

	assert.Equal(t, output, received)
}

// TestWaitPtyDrainedBackgroundWriter covers a background process which keeps writing to the pty
// after the child exited: the session does not wait for it to stop.
func TestWaitPtyDrainedBackgroundWriter(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		interval time.Duration
		stdout   io.Writer
	}{
		{name: "ticking", interval: 100 * time.Millisecond, stdout: io.Discard},
		{name: "without pause, slow client", stdout: slowWriter{delay: 5 * time.Millisecond}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			master, slave := openTestPty(t)

			go func() {
				chunk := []byte(strings.Repeat("tick\n", 512))

				for t.Context().Err() == nil {
					if _, err := slave.Write(chunk); err != nil {
						return
					}

					time.Sleep(test.interval)
				}
			}()

			var childExited atomic.Bool

			relayDone := make(chan struct{})

			go func() {
				defer close(relayDone)

				relayPty(master, test.stdout, &childExited)
			}()

			start := time.Now()

			waitPtyDrained(t.Context(), master, &childExited, relayDone)

			require.Less(t, time.Since(start), ptyDrainTimeout+5*time.Second)
		})
	}
}

type slowWriter struct {
	delay time.Duration
}

func (w slowWriter) Write(p []byte) (int, error) {
	time.Sleep(w.delay)

	return len(p), nil
}

// TestWaitPtyDrainedRetainedPty covers a background process which keeps the pty open without
// writing anything: the session does not wait for it.
func TestWaitPtyDrainedRetainedPty(t *testing.T) {
	t.Parallel()

	master, _ := openTestPty(t)

	var childExited atomic.Bool

	relayDone := make(chan struct{})

	go func() {
		defer close(relayDone)

		relayPty(master, io.Discard, &childExited)
	}()

	start := time.Now()

	waitPtyDrained(t.Context(), master, &childExited, relayDone)

	require.Less(t, time.Since(start), ptyDrainTimeout+5*time.Second)

	select {
	case <-relayDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the relay is still blocked on the idle pty")
	}
}
