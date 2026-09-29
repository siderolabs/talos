// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

// Package ctrltrace implements a debug-only tracer for the COSI controller runtime.
//
// When enabled, it records every resource operation performed by the controllers
// (and the controller lifecycle events) into a JSONL file which can be pulled off
// the node and replayed as an animation of the boot process.
//
// This file is the `sidero.debug` build (`WITH_DEBUG=1`); production builds get the
// inert stub instead. Even in a debug build the tracer stays off until it is turned on
// via the kernel command line (`talos.trace=1`) or the `TALOS_TRACE` environment
// variable, and it costs nothing until then: the state and the controllers are not wrapped.
package ctrltrace

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/siderolabs/go-procfs/procfs"
)

// Kernel command line / environment knobs.
const (
	// KernelParamTrace enables the controller runtime tracer: `talos.trace=1`.
	KernelParamTrace = "talos.trace"
	// KernelParamTracePath overrides the output path: `talos.trace.path=/run/foo.jsonl`.
	KernelParamTracePath = "talos.trace.path"
	// KernelParamTraceSpecs records resource specs on create/update: `talos.trace.specs=1`.
	KernelParamTraceSpecs = "talos.trace.specs"
	// KernelParamTraceDuration stops tracing after the given duration: `talos.trace.duration=5m`.
	KernelParamTraceDuration = "talos.trace.duration"

	// EnvTrace is the environment variable equivalent of KernelParamTrace (useful in containers).
	EnvTrace = "TALOS_TRACE"

	// DefaultPath is the default trace output path.
	DefaultPath = "/run/talos-trace.jsonl"
)

// FormatVersion is the version of the trace file format.
const FormatVersion = 1

const (
	queueSize   = 1 << 16
	flushPeriod = 250 * time.Millisecond
)

// Event is a single record in the trace.
//
// Field names are kept short: a full boot produces a lot of these.
type Event struct {
	// T is the offset from the trace start, in microseconds.
	T int64 `json:"t"`
	// K is the record kind: "op", "ctrl", "meta" or "graph".
	K string `json:"k"`
	// Op is the operation: get/list/create/update/destroy/watch/watchkind for "op" records,
	// start/wake/queue/inputs/stop/crash for "ctrl" records.
	Op string `json:"op"`
	// Ctrl is the name of the caller (controller name, or "runtime" for the runtime itself).
	Ctrl string `json:"ctrl,omitempty"`

	NS string `json:"ns,omitempty"`
	RT string `json:"rt,omitempty"`
	ID string `json:"id,omitempty"`

	Owner string   `json:"own,omitempty"`
	Ver   string   `json:"ver,omitempty"`
	Phase string   `json:"ph,omitempty"`
	Fin   []string `json:"fin,omitempty"`

	// N is the number of items returned by a list.
	N int `json:"n,omitempty"`
	// Dur is the call duration in microseconds.
	Dur int64 `json:"dur,omitempty"`

	Err  string `json:"err,omitempty"`
	Spec string `json:"spec,omitempty"`

	// Extra holds arbitrary payload for "meta" and "graph" records.
	Extra any `json:"x,omitempty"`
}

// Tracer collects and persists trace events.
type Tracer struct {
	start     time.Time
	ch        chan Event
	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup

	dropped atomic.Int64
	written atomic.Int64

	withSpecs bool
	path      string
}

//nolint:gochecknoglobals
var global atomic.Pointer[Tracer]

// Enabled reports whether tracing is on.
//
// This is a single atomic load, cheap enough for the hot path.
func Enabled() bool {
	return global.Load() != nil
}

// Global returns the active tracer, or nil.
func Global() *Tracer {
	return global.Load()
}

// Options configures the tracer.
type Options struct {
	// Path is the trace output file.
	Path string
	// TalosVersion is recorded in the trace header.
	TalosVersion string
	// Duration, if non-zero, stops tracing after that much time.
	Duration time.Duration
	// WithSpecs records resource specs on create/update.
	WithSpecs bool
}

// Setup initializes the global tracer if it is enabled via kernel cmdline or environment.
//
// It returns nil when tracing is disabled; every method on a nil *Tracer is a no-op.
func Setup(talosVersion string) *Tracer {
	if !enabledFromParams() {
		return nil
	}

	opts := Options{
		Path:         DefaultPath,
		TalosVersion: talosVersion,
		WithSpecs:    truthy(param(KernelParamTraceSpecs)),
	}

	if v := param(KernelParamTracePath); v != "" {
		opts.Path = v
	}

	if v := param(KernelParamTraceDuration); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			opts.Duration = d
		}
	}

	return New(opts)
}

// New starts a tracer and installs it as the global one.
func New(opts Options) *Tracer {
	if opts.Path == "" {
		opts.Path = DefaultPath
	}

	t := &Tracer{
		start:     time.Now(),
		ch:        make(chan Event, queueSize),
		done:      make(chan struct{}),
		withSpecs: opts.WithSpecs,
		path:      opts.Path,
	}

	t.wg.Add(1)

	go t.run()

	global.Store(t)

	t.Emit(Event{
		K:  "meta",
		Op: "start",
		Extra: map[string]any{
			"version":       FormatVersion,
			"talos_version": opts.TalosVersion,
			"wall_clock":    t.start.UTC().Format(time.RFC3339Nano),
			"with_specs":    t.withSpecs,
		},
	})

	if opts.Duration > 0 {
		time.AfterFunc(opts.Duration, func() { t.Close() }) //nolint:errcheck
	}

	return t
}

// Path returns the file the trace is written to.
func (t *Tracer) Path() string {
	return t.path
}

// WithSpecs reports whether resource specs are recorded.
func (t *Tracer) WithSpecs() bool {
	return t.withSpecs
}

// Emit records an event, dropping it if the queue is full.
//
// Emit never blocks: tracing must not change the timing of what it observes more
// than it already does.
func (t *Tracer) Emit(ev Event) {
	if t == nil {
		return
	}

	ev.T = time.Since(t.start).Microseconds()

	select {
	case t.ch <- ev:
	default:
		t.dropped.Add(1)
	}
}

// Close flushes and closes the trace, and disables further tracing.
func (t *Tracer) Close() error {
	if t == nil {
		return nil
	}

	t.closeOnce.Do(func() {
		global.CompareAndSwap(t, nil)
		close(t.done)
	})

	t.wg.Wait()

	return nil
}

// Stats returns the number of written and dropped events.
func (t *Tracer) Stats() (written, dropped int64) {
	if t == nil {
		return 0, 0
	}

	return t.written.Load(), t.dropped.Load()
}

func (t *Tracer) run() {
	defer t.wg.Done()

	f := t.openOutput()
	if f == nil {
		return
	}

	defer f.Close() //nolint:errcheck

	w := bufio.NewWriterSize(f, 256*1024)
	defer w.Flush() //nolint:errcheck

	enc := json.NewEncoder(w)

	ticker := time.NewTicker(flushPeriod)
	defer ticker.Stop()

	write := func(ev Event) {
		if enc.Encode(&ev) == nil {
			t.written.Add(1)
		}
	}

	for {
		select {
		case ev := <-t.ch:
			write(ev)
		case <-ticker.C:
			w.Flush() //nolint:errcheck
		case <-t.done:
			// drain whatever is left without blocking
			for {
				select {
				case ev := <-t.ch:
					write(ev)

					continue
				default:
				}

				break
			}

			written, dropped := t.Stats()

			write(Event{
				T:  time.Since(t.start).Microseconds(),
				K:  "meta",
				Op: "stop",
				Extra: map[string]any{
					"written": written,
					"dropped": dropped,
				},
			})

			return
		}
	}
}

// openOutput waits for the output path to become writable.
//
// Tracing starts before /run is mounted, so the first events are buffered in the
// queue while we keep retrying; events are only lost if the queue overflows first.
func (t *Tracer) openOutput() *os.File {
	const retryPeriod = 100 * time.Millisecond

	ticker := time.NewTicker(retryPeriod)
	defer ticker.Stop()

	for {
		f, err := os.OpenFile(t.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err == nil {
			return f
		}

		select {
		case <-t.done:
			return nil
		case <-ticker.C:
		}
	}
}

func enabledFromParams() bool {
	return truthy(param(KernelParamTrace)) || truthy(os.Getenv(EnvTrace))
}

func param(name string) string {
	if p := procfs.ProcCmdline().Get(name).First(); p != nil {
		return *p
	}

	return ""
}

func truthy(v string) bool {
	if v == "" {
		return false
	}

	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}

	return v == "on" || v == "yes"
}

// Ctrl records a controller lifecycle event.
func (t *Tracer) Ctrl(name, op string, err error) {
	if t == nil {
		return
	}

	ev := Event{K: "ctrl", Op: op, Ctrl: name}
	if err != nil {
		ev.Err = err.Error()
	}

	t.Emit(ev)
}

type callerKey struct{}

// WithCaller tags the context with the name of the controller making the calls.
//
// The tag travels with the context all the way down to the state wrapper, which is
// what lets the tracer attribute reads (and not just writes) to a controller.
func WithCaller(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, callerKey{}, name)
}

// CallerFrom returns the controller name stored in the context, if any.
func CallerFrom(ctx context.Context) string {
	name, _ := ctx.Value(callerKey{}).(string)

	return name
}
