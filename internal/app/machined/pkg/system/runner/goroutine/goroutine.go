/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package goroutine

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/log"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/config"
)

// goroutineRunner is a runner.Runner that runs a service in a goroutine
type goroutineRunner struct {
	main   FuncMain
	id     string
	config config.Configurator

	opts *runner.Options

	ctx       context.Context
	ctxCancel context.CancelFunc

	wg sync.WaitGroup
}

// FuncMain is a entrypoint into the service.
//
// Service should abort and return when ctx is canceled
type FuncMain func(ctx context.Context, config config.Configurator, logOutput io.Writer) error

// NewRunner creates runner.Runner that runs a service as goroutine
func NewRunner(config config.Configurator, id string, main FuncMain, setters ...runner.Option) runner.Runner {
	r := &goroutineRunner{
		id:     id,
		config: config,
		main:   main,
		opts:   runner.DefaultOptions(),
	}

	r.ctx, r.ctxCancel = context.WithCancel(context.Background())

	for _, setter := range setters {
		setter(r.opts)
	}

	return r
}

// Open implements the Runner interface.
func (r *goroutineRunner) Open(ctx context.Context) error {
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
			n := runtime.Stack(buf, false)
			err = errors.Errorf("panic in service: %v\n%s", r, string(buf[:n]))
		}
	}()

	var w *log.Log

	w, err = log.New(r.id, r.opts.LogPath)
	if err != nil {
		err = errors.Wrap(err, "service log handler")
		return
	}
	// nolint: errcheck
	defer w.Close()

	var writer io.Writer
	if r.config.Debug() {
		writer = io.MultiWriter(w, os.Stdout)
	} else {
		writer = w
	}

	err = r.main(r.ctx, r.config, writer)
	if err == context.Canceled {
		// clear error if service was aborted
		err = nil
	}

	return err
}

// Stop implements the Runner interface
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
