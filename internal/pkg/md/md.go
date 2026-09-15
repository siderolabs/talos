// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package md provides a Go interface to Linux MD (software RAID) arrays via
// the mdadm(8) utility.
package md

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

// MD provides methods for managing MD (software RAID) arrays.
type MD struct {
	mdadm string
}

// New creates a new MD instance, resolving the mdadm binary.
func New(opts ...Option) (*MD, error) {
	md := &MD{
		mdadm: "/sbin/mdadm",
	}

	for _, opt := range opts {
		opt(md)
	}

	return md, nil
}

// Option is a functional option for configuring the MD instance.
type Option func(*MD)

// WithMdadmPath sets an explicit path to the mdadm binary.
func WithMdadmPath(path string) Option {
	return func(md *MD) {
		md.mdadm = path
	}
}

// run executes `mdadm <args...>` and returns stdout.
//
// Errors are normalised through classifyError so every caller sees the same
// sentinel set (ErrNotFound, ErrInUse, ErrExists, ErrCommand). The raw mdadm
// stderr is kept out of the returned error chain - only the sentinel is
// wrapped - so it will not be surfaced to API clients by mistake.
func (md *MD) run(ctx context.Context, args ...string) (string, error) {
	out, err := cmd.RunWithOptions(ctx, md.mdadm, args, cmd.WithFullStdoutCapture())
	if err != nil {
		return "", fmt.Errorf("mdadm failed: %w", classifyError(err))
	}

	return out, nil
}

// EventCallback is a thread-safe callback for handling events of type T.
type EventCallback[T any] struct {
	mu      sync.Mutex
	onEvent func(T)
}

// NewEventCallback creates a new EventCallback for handling events of type T.
func NewEventCallback[T any](onEvent func(T)) *EventCallback[T] {
	return &EventCallback[T]{
		onEvent: onEvent,
	}
}

// Emit calls the onEvent callback with the provided event of type T.
func (ec *EventCallback[T]) Emit(event T) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.onEvent(event)
}

const (
	// monitorStderrLimit caps the stderr kept for classifying a monitor exit.
	// mdadm prints the reason for exiting last, so only the tail is kept.
	monitorStderrLimit = 64 * 1024

	// monitorLineLimit caps a single buffered output line. The rest of a longer
	// line is dropped.
	monitorLineLimit = 4 * 1024
)

// Monitor runs mdadm monitor and calls onEvent for each emitted event line.
//
// onEvent receives stdout lines and recognized stderr event lines: mdadm's
// monitor emits events on stderr. Other stderr remains classification-only, so
// the terminal "no array" condition surfaces once, as the quiet ErrNotFound
// return value, rather than also being logged per restart. Only the last
// monitorStderrLimit bytes of stderr are kept for that classification.
func (md *MD) Monitor(ctx context.Context, onEvent func(string)) error {
	stderr := newTailBuffer(monitorStderrLimit)

	ec := NewEventCallback(onEvent)

	process, err := cmd.StartWithOptions(
		ctx,
		md.mdadm,
		[]string{"--monitor", "--scan", "--mail=talos@local"},
		cmd.WithStdout(newLineWriter(func(s string) {
			ec.Emit(s)
		})),
		cmd.WithStderr(newLineWriter(func(s string) {
			stderr.WriteLine(s)

			if strings.Contains(s, " event detected ") {
				ec.Emit(s)
			}
		})),
	)
	if err != nil {
		return fmt.Errorf("start mdadm monitor: %w", err)
	}

	if err = process.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}

		return fmt.Errorf("mdadm monitor failed: %w", classifyProcessError(err, stderr.Bytes()))
	}

	return nil
}

type lineWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	onEvent func(string)
}

func newLineWriter(onEvent func(string)) *lineWriter {
	return &lineWriter{onEvent: onEvent}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	n := len(p)

	w.mu.Lock()
	defer w.mu.Unlock()

	for len(p) > 0 {
		if i := bytes.IndexByte(p, '\n'); i >= 0 {
			w.append(p[:i])
			w.onEvent(w.buf.String())
			w.buf.Reset()

			p = p[i+1:]

			continue
		}

		w.append(p)

		break
	}

	return n, nil
}

// append adds p to the current line, dropping whatever exceeds monitorLineLimit.
func (w *lineWriter) append(p []byte) {
	if room := monitorLineLimit - w.buf.Len(); len(p) > room {
		p = p[:max(room, 0)]
	}

	w.buf.Write(p)
}

// tailBuffer keeps the last limit bytes of whole lines written to it.
type tailBuffer struct {
	limit int
	buf   []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

// WriteLine appends s and a newline, then drops the oldest lines beyond the limit.
func (b *tailBuffer) WriteLine(s string) {
	b.buf = append(b.buf, s...)
	b.buf = append(b.buf, '\n')

	if excess := len(b.buf) - b.limit; excess > 0 {
		// drop through the end of the line that the cut lands in
		if i := bytes.IndexByte(b.buf[excess:], '\n'); i >= 0 && excess+i+1 < len(b.buf) {
			excess += i + 1
		}

		b.buf = append(b.buf[:0], b.buf[excess:]...)
	}
}

// Bytes returns the kept tail.
func (b *tailBuffer) Bytes() []byte {
	return b.buf
}

func classifyProcessError(err error, stderr []byte) error {
	var exit *cmd.ExitError
	if !errors.As(err, &exit) {
		return err
	}

	return &ExecError{
		Sentinel: sentinelFor(&cmd.ExitError{Output: stderr}),
		ExitCode: exit.ExitCode,
		Stderr:   stderr,
	}
}
