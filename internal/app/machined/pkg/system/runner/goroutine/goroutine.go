// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package goroutine

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdlibruntime "runtime"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
)

// ErrAborted is returned by the service when it's aborted (doesn't stop on timeout).
var ErrAborted = errors.New("service aborted")

// goroutineRunner is a runner.Runner that runs a service in a goroutine.
type goroutineRunner struct {
	main    FuncMain
	id      string
	runtime runtime.Runtime

	opts *runner.Options
}

// FuncMain is a entrypoint into the service.
//
// Service should abort and return when ctx is canceled.
type FuncMain func(ctx context.Context, r runtime.Runtime, logOutput io.Writer) error

// NewRunner creates runner.Runner that runs a service as goroutine.
func NewRunner(r runtime.Runtime, id string, main FuncMain, setters ...runner.Option) runner.Runner {
	run := &goroutineRunner{
		id:      id,
		runtime: r,
		main:    main,
		opts:    runner.DefaultOptions(),
	}

	for _, setter := range setters {
		setter(run.opts)
	}

	return run
}

// Open implements the Runner interface.
func (r *goroutineRunner) Open() error {
	return nil
}

// Run implements the Runner interface.
//
// A goroutine service shares the machined process, so there is no exit code of its own to report:
// how it ended is carried by the error.
func (r *goroutineRunner) Run(ctx context.Context, eventSink events.Recorder, onStart runner.OnStart) (runner.Status, error) {
	status := runner.Status{Started: true}

	if onStart != nil {
		onStart(int32(unix.Getpid()))
	}

	eventSink(events.StateRunning, "Service started as goroutine")

	errCh := make(chan error)

	go func() {
		errCh <- r.wrappedMain(ctx)
	}()

	select {
	case <-ctx.Done():
		eventSink(events.StateStopping, "Service stopping")
	case err := <-errCh:
		// service finished on its own
		return status, err
	}

	select {
	case <-time.After(r.opts.GracefulShutdownTimeout * 2):
		eventSink(events.StateStopping, "Service hasn't stopped gracefully on timeout, aborting")

		return status, ErrAborted
	case err := <-errCh:
		return status, err
	}
}

func (r *goroutineRunner) wrappedMain(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 8192)
			n := stdlibruntime.Stack(buf, false)
			err = fmt.Errorf("panic in service: %v\n%s", r, string(buf[:n]))
		}
	}()

	w, err := r.opts.LoggingManager.ServiceLog(r.id).Writer()
	if err != nil {
		return fmt.Errorf("service log handler: %w", err)
	}

	writerCloser := sync.OnceValue(w.Close)

	defer writerCloser() //nolint:errcheck

	if err = r.main(ctx, r.runtime, w); !errors.Is(err, context.Canceled) {
		return err // return error if it's not context.Canceled (service was not aborted)
	}

	return writerCloser()
}

// Close implements the Runner interface.
func (r *goroutineRunner) Close() error {
	return nil
}

func (r *goroutineRunner) String() string {
	return fmt.Sprintf("Goroutine(%q)", r.id)
}
