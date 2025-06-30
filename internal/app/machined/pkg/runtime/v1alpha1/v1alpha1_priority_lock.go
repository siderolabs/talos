// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// Priority describes the running priority of a process.
//
// If CanTakeOver returns true, current process with "lower" priority
// will be canceled and "higher" priority process will be run.
type Priority[T any] interface {
	comparable
	CanTakeOver(another T) bool
}

// PriorityLock is a lock that makes sure that only a single process can run at a time.
//
// If a process with "higher" priority tries to acquire the lock, previous process is stopped
// and new process with "higher" priority is run.
type PriorityLock[T Priority[T]] struct {
	runningCh  chan struct{}
	takeoverCh chan struct{}

	mu              sync.Mutex
	runningPriority T
	cancelCtx       context.CancelFunc
}

// NewPriorityLock returns a new PriorityLock.
func NewPriorityLock[T Priority[T]]() *PriorityLock[T] {
	runningCh := make(chan struct{}, 1)
	runningCh <- struct{}{}

	return &PriorityLock[T]{
		runningCh:  runningCh,
		takeoverCh: make(chan struct{}, 1),
	}
}

func (lock *PriorityLock[T]) getRunningPriority() (T, context.CancelFunc) {
	lock.mu.Lock()
	defer lock.mu.Unlock()

	return lock.runningPriority, lock.cancelCtx
}

func (lock *PriorityLock[T]) setRunningPriority(seq T, cancelCtx context.CancelFunc) {
	lock.mu.Lock()
	defer lock.mu.Unlock()

	var zeroSeq T

	if seq == zeroSeq && lock.cancelCtx != nil {
		lock.cancelCtx()
	}

	lock.runningPriority, lock.cancelCtx = seq, cancelCtx
}

// Lock acquires the lock according the priority rules and returns a context that should be used within the process.
//
// Process should terminate as soon as the context is canceled.
// Argument seq defines the priority of the process.
// Argument takeOverTimeout defines the maximum time to wait for the low-priority process to terminate.
func (lock *PriorityLock[T]) Lock(ctx context.Context, takeOverTimeout time.Duration, seq T, options ...runtime.LockOption) (context.Context, error) {
	opts := runtime.DefaultControllerOptions()
	for _, o := range options {
		if err := o(&opts); err != nil {
			return nil, err
		}
	}

	takeOverTimer := time.NewTimer(takeOverTimeout)
	defer takeOverTimer.Stop()

	select {
	case lock.takeoverCh <- struct{}{}:
	case <-takeOverTimer.C:
		return nil, errors.New("failed to acquire lock: timeout")
	}

	defer func() {
		<-lock.takeoverCh
	}()

	sequence, cancelCtx := lock.getRunningPriority()

	if !seq.CanTakeOver(sequence) && !opts.Takeover {
		return nil, runtime.ErrLocked
	}

	if cancelCtx != nil {
		cancelCtx()
	}

	select {
	case <-lock.runningCh:
		seqCtx, seqCancel := context.WithCancel(ctx)
		lock.setRunningPriority(seq, seqCancel)

		return seqCtx, nil
	case <-takeOverTimer.C:
		return nil, errors.New("failed to acquire lock: timeout")
	}
}

// Unlock releases the lock.
func (lock *PriorityLock[T]) Unlock() {
	var zeroSeq T

	lock.setRunningPriority(zeroSeq, nil)

	lock.runningCh <- struct{}{}
}
