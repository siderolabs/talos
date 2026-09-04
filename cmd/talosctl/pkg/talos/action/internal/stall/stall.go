// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package stall detects a lack of progress in an operation which reports no error of its own.
//
// A Talos API connection whose peer went away without closing it gracefully stays `READY` and
// delivers nothing: long-running calls are mostly server-streaming, so nothing is ever written
// that could draw a RST back, and the caller blocks with no error to react to.
//
// Waiting on such a call has to be bounded by observing that nothing is arriving, which is what a Detector does.
package stall

import (
	"context"
	"sync/atomic"
	"time"
)

// Detector invokes onStall unless it is poked at least once every timeout.
//
// It stops once it fires, or once its context is done.
type Detector struct {
	pokeCh chan struct{}
	fired  atomic.Bool
}

// NewDetector starts a Detector which calls onStall if it is not poked within timeout.
func NewDetector(ctx context.Context, timeout time.Duration, onStall func()) *Detector {
	d := &Detector{
		pokeCh: make(chan struct{}, 1),
	}

	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-d.pokeCh:
				timer.Reset(timeout)
			case <-timer.C:
				d.fired.Store(true)

				onStall()

				return
			}
		}
	}()

	return d
}

// Poke records progress, restarting the countdown.
func (d *Detector) Poke() {
	select {
	case d.pokeCh <- struct{}{}:
	default:
	}
}

// Stalled reports whether the detector has fired.
//
// It lets a caller tell an error caused by the detector's own onStall from an unrelated failure.
func (d *Detector) Stalled() bool {
	return d.fired.Load()
}
