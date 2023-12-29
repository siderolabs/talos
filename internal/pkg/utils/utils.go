// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package utils provides various utility functions.
package utils

import (
	"errors"
	"slices"
	"sync/atomic"

	"github.com/siderolabs/gen/pair"
)

// UpdatePairSet updates a set of pairs. It removes pairs that are not in toAdd and adds pairs that are not in old.
func UpdatePairSet[T comparable, H any](
	old []pair.Pair[T, H],
	toAdd []T,
	add func(T) (H, error),
	remove func(pair.Pair[T, H]) error,
) ([]pair.Pair[T, H], error) {
	var err error

	result := slices.DeleteFunc(old, func(h pair.Pair[T, H]) bool {
		if err != nil {
			return false
		}

		if slices.Contains(toAdd, h.F1) {
			return false
		}

		err = remove(h)
		if err != nil { //nolint:gosimple
			return false
		}

		return true
	})

	if err != nil {
		return result, err
	}

	for _, val := range toAdd {
		if slices.ContainsFunc(old, func(h pair.Pair[T, H]) bool { return h.F1 == val }) {
			continue
		}

		h, err := add(val)
		if err != nil {
			return result, err
		}

		result = append(result, pair.MakePair(val, h))
	}

	return result, nil
}

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
