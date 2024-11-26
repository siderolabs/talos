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

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
)

// goroutineRunner is a runner.Runner that runs a service in a goroutine.
type goroutineRunner struct {
	main    FuncMain
	id      string
	runtime runtime.Runtime

	opts *runner.Options

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	wg sync.WaitGroup
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

	run.ctx, run.ctxCancel = context.WithCancel(context.Background())

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
func (r *goroutineRunner) Run(eventSink events.Recorder) error {
	r.wg.Add(1)
	defer r.wg.Done()

	eventSink(events.StateRunning, "Service started as goroutine")

	return r.wrappedMain()
}

func (r *goroutineRunner) wrappedMain() (err error) {
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

	if err = r.main(r.ctx, r.runtime, w); !errors.Is(err, context.Canceled) {
		return err // return error if it's not context.Canceled (service was not aborted)
	}

	return writerCloser()
}

// Stop implements the Runner interface.
func (r *goroutineRunner) Stop() error {
	r.ctxCancel()

	r.wg.Wait()

	r.ctx, r.ctxCancel = context.WithCancel(context.Background())

	return nil
}

// Close implements the Runner interface.
func (r *goroutineRunner) Close() error {
	return nil
}

func (r *goroutineRunner) String() string {
	return fmt.Sprintf("Goroutine(%q)", r.id)
}
