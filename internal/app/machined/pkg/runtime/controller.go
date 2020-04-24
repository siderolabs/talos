// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/api/machine"
)

// TaskSetupFunc defines the function that a task will execute for a specific runtime
// mode.
type TaskSetupFunc func(data interface{}) TaskExecutionFunc

// TaskExecutionFunc defines the function that a task will execute for a specific runtime
// mode.
type TaskExecutionFunc func(context.Context, *log.Logger, Runtime) error

// Phase represents a collection of tasks to be performed concurrently.
type Phase []TaskSetupFunc

// Controller represents the controller responsible for managing the execution
// of sequences.
type Controller struct {
	Runtime   Runtime
	Sequencer Sequencer

	semaphore int32
}

// Run executes all phases known to the controller in serial. `Controller`
// aborts immediately if any phase fails.
func (c *Controller) Run(seq Sequence, data interface{}) error {
	// We must ensure that the runtime is configured since all sequences depend
	// on the runtime.
	if c.Runtime == nil {
		return ErrUndefinedRuntime
	}

	// Allow only one sequence to run at a time.
	if c.TryLock() {
		return ErrLocked
	}

	defer c.Unlock()

	phases, err := c.phases(seq, data)
	if err != nil {
		return err
	}

	return c.run(seq, phases, data)
}

// TryLock attempts to set a lock that prevents multiple sequences from running
// at once. If currently locked, a value of true will be returned. If not
// currently locked, a value of false will be returned.
func (c *Controller) TryLock() bool {
	return !atomic.CompareAndSwapInt32(&c.semaphore, 0, 1)
}

// Unlock removes the lock set by `TryLock`.
func (c *Controller) Unlock() bool {
	return atomic.CompareAndSwapInt32(&c.semaphore, 1, 0)
}

func (c *Controller) run(seq Sequence, phases []Phase, data interface{}) error {
	start := time.Now()

	log.Printf("sequence [%s]: %d phase(s)", seq.String(), len(phases))
	defer log.Printf("sequence [%s]: done: %s", seq.String(), time.Since(start))

	var (
		number int
		phase  Phase
		err    error
	)

	for number, phase = range phases {
		// Make the phase number human friendly.
		number++

		func() {
			start := time.Now()

			progress := fmt.Sprintf("%d/%d", number, len(phases))

			log.Printf("phase [%s]: starting", progress)
			defer log.Printf("phase [%s]: done, %s", progress, time.Since(start))

			if err = c.runPhase(phase, data); err != nil {
				return
			}
		}()
	}

	if err != nil {
		return fmt.Errorf("error running phase %d in %s sequence: %w", number, seq.String(), err)
	}

	return nil
}

func (c *Controller) runPhase(phase Phase, data interface{}) error {
	var eg errgroup.Group

	for number, task := range phase {
		// Make the task number human friendly.
		number := number

		number++

		task := task

		eg.Go(func() error {
			start := time.Now()

			progress := fmt.Sprintf("%d/%d", number, len(phase))

			log.Printf("task [%s]: starting", progress)
			defer log.Printf("task [%s]: done, %s", progress, time.Since(start))

			if err := c.runTask(number, task, data); err != nil {
				return fmt.Errorf("task [%s]: failed, %w", progress, err)
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (c *Controller) runTask(n int, f TaskSetupFunc, data interface{}) error {
	logger := log.New(os.Stdout, fmt.Sprintf("task %d", n), 0)

	if task := f(data); task != nil {
		return task(context.TODO(), logger, c.Runtime)
	}

	return nil
}

func (c *Controller) phases(seq Sequence, data interface{}) ([]Phase, error) {
	var phases []Phase

	switch seq {
	case Boot:
		phases = c.Sequencer.Boot(c.Runtime)
	case Initialize:
		phases = c.Sequencer.Initialize(c.Runtime)
	case Shutdown:
		phases = c.Sequencer.Shutdown(c.Runtime)
	case Reboot:
		phases = c.Sequencer.Reboot(c.Runtime)
	case Upgrade:
		var (
			in *machine.UpgradeRequest
			ok bool
		)

		if in, ok = data.(*machine.UpgradeRequest); !ok {
			return nil, ErrInvalidSequenceData
		}

		phases = c.Sequencer.Upgrade(c.Runtime, in)
	case Reset:
		var (
			in *machine.ResetRequest
			ok bool
		)

		if in, ok = data.(*machine.ResetRequest); !ok {
			return nil, ErrInvalidSequenceData
		}

		phases = c.Sequencer.Reset(c.Runtime, in)
	}

	return phases, nil
}
