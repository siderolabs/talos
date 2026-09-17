// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import "time"

const (
	defaultLVMRetryInitialInterval = time.Second
	defaultLVMRetryMaxInterval     = 30 * time.Second
)

type lvmReconcileRetry struct {
	timer   *time.Timer
	initial time.Duration
	maximum time.Duration
	next    time.Duration
	armed   bool
}

func newLVMReconcileRetry(initial, maximum time.Duration) *lvmReconcileRetry {
	if initial <= 0 {
		initial = defaultLVMRetryInitialInterval
	}

	if maximum < initial {
		maximum = defaultLVMRetryMaxInterval
		if maximum < initial {
			maximum = initial
		}
	}

	timer := time.NewTimer(initial)
	if !timer.Stop() {
		<-timer.C
	}

	return &lvmReconcileRetry{timer: timer, initial: initial, maximum: maximum, next: initial}
}

func (retry *lvmReconcileRetry) channel() <-chan time.Time {
	return retry.timer.C
}

func (retry *lvmReconcileRetry) schedule() {
	if retry.armed {
		return
	}

	retry.timer.Reset(retry.next)
	retry.armed = true
	retry.next = min(retry.next*2, retry.maximum)
}

func (retry *lvmReconcileRetry) fired() {
	retry.armed = false
}

func (retry *lvmReconcileRetry) reset() {
	if retry.armed && !retry.timer.Stop() {
		select {
		case <-retry.timer.C:
		default:
		}
	}

	retry.armed = false
	retry.next = retry.initial
}

func (retry *lvmReconcileRetry) stop() {
	retry.timer.Stop()
}
