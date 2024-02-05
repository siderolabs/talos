// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package utils provides various utility functions.
package utils

import (
	"errors"
	"sync/atomic"
)

const (
	notRunning = iota
	running
	closing
	closed
)

// Runner is a fn/stop runner.
type Runner struct {
	fn        func() error
	stop      func() error
	retryStop func(error) bool
	status    atomic.Int64
	done      chan struct{}
}

// NewRunner creates a new runner.
func NewRunner(fn, stop func() error, retryStop func(error) bool) *Runner {
	return &Runner{fn: fn, stop: stop, retryStop: retryStop, done: make(chan struct{})}
}

// Run runs fn.
func (r *Runner) Run() error {
	defer func() {
		if r.status.Swap(closed) != closed {
			close(r.done)
		}
	}()

	if !r.status.CompareAndSwap(notRunning, running) {
		return ErrAlreadyRunning
	}

	return r.fn()
}

var (
	// ErrAlreadyRunning is the error that is returned when runner is already running/closing/closed.
	ErrAlreadyRunning = errors.New("runner is already running/closing/closed")
	// ErrNotRunning is the error that is returned when runner is not running/closing/closed.
	ErrNotRunning = errors.New("runner is not running/closing/closed")
)

// Stop stops runner. It's safe to call even if runner is already stopped or in process of being stopped.
func (r *Runner) Stop() error {
	if r.status.CompareAndSwap(notRunning, closing) || !r.status.CompareAndSwap(running, closing) {
		return ErrNotRunning
	}

	for {
		err := r.stop()
		if err != nil {
			if r.retryStop(err) && r.status.Load() == closing {
				continue
			}
		}

		<-r.done

		return err
	}
}
