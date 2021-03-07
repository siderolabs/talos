// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package restart

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
)

type restarter struct {
	wrappedRunner runner.Runner
	opts          *Options

	stop    chan struct{}
	stopped chan struct{}
}

// New wraps runner.Runner with restart policy.
func New(wrapRunner runner.Runner, opts ...Option) runner.Runner {
	r := &restarter{
		wrappedRunner: wrapRunner,
		opts:          DefaultOptions(),
		stop:          make(chan struct{}),
		stopped:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(r.opts)
	}

	return r
}

// Options is the functional options struct.
type Options struct {
	// Type describes the service's restart policy.
	Type Type
	// RestartInterval is the interval between restarts for failed runs.
	RestartInterval time.Duration
}

// Option is the functional option func.
type Option func(*Options)

// Type represents the service's restart policy.
type Type int

const (
	// Forever will always restart a process.
	Forever Type = iota
	// Once will run process exactly once.
	Once
	// UntilSuccess will restart process until run succeeds.
	UntilSuccess
)

func (t Type) String() string {
	switch t {
	case Forever:
		return "Forever"
	case Once:
		return "Once"
	case UntilSuccess:
		return "UntilSuccess"
	default:
		return "Unknown"
	}
}

// DefaultOptions describes the default options to a runner.
func DefaultOptions() *Options {
	return &Options{
		Type:            Forever,
		RestartInterval: 5 * time.Second,
	}
}

// WithType sets the type of a service.
func WithType(o Type) Option {
	return func(args *Options) {
		args.Type = o
	}
}

// WithRestartInterval sets the interval between restarts of the failed task.
func WithRestartInterval(interval time.Duration) Option {
	return func(args *Options) {
		args.RestartInterval = interval
	}
}

// Open implements the Runner interface.
func (r *restarter) Open(ctx context.Context) error {
	return r.wrappedRunner.Open(ctx)
}

// Run implements the Runner interface
//nolint:gocyclo
func (r *restarter) Run(eventSink events.Recorder) error {
	defer close(r.stopped)

	for {
		errCh := make(chan error)

		go func() {
			errCh <- r.wrappedRunner.Run(eventSink)
		}()

		var err error

		select {
		case <-r.stop:
			//nolint:errcheck
			_ = r.wrappedRunner.Stop()

			return <-errCh
		case err = <-errCh:
		}

		errStop := r.wrappedRunner.Stop()
		if errStop != nil {
			return errStop
		}

		switch r.opts.Type {
		case Once:
			return err
		case UntilSuccess:
			if err == nil {
				return nil
			}

			eventSink(events.StateWaiting, "Error running %s, going to restart until it succeeds: %v", r.wrappedRunner, err)
		case Forever:
			if err == nil {
				eventSink(events.StateWaiting, "Runner %s exited without error, going to restart it", r.wrappedRunner)
			} else {
				eventSink(events.StateWaiting, "Error running %v, going to restart forever: %v", r.wrappedRunner, err)
			}
		}

		select {
		case <-r.stop:
			eventSink(events.StateStopping, "Aborting restart sequence")

			return nil
		case <-time.After(r.opts.RestartInterval):
		}
	}
}

// Stop implements the Runner interface.
func (r *restarter) Stop() error {
	close(r.stop)

	<-r.stopped

	return nil
}

// Close implements the Runner interface.
func (r *restarter) Close() error {
	return r.wrappedRunner.Close()
}

// String implements the Runner interface.
func (r *restarter) String() string {
	return fmt.Sprintf("Restart(%s, %s)", r.opts.Type, r.wrappedRunner)
}
