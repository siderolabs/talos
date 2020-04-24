// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/api/machine"
)

// TaskFunc defines the function that a task will execute for a specific runtime
// mode.
type TaskFunc func(Runtime) error

// Task represents a task within a phase.
type Task interface {
	Func(Mode) TaskFunc
}

// Phase represents a collection of tasks to be performed concurrently.
type Phase interface {
	Tasks() []Task
}

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

	return c.run(seq, phases)
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

func (c *Controller) run(seq Sequence, phases []Phase) error {
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

			if err = c.runPhase(phase); err != nil {
				return
			}
		}()
	}

	if err != nil {
		return fmt.Errorf("error running phase %d in %s sequence: %w", number, seq.String(), err)
	}

	return nil
}

func (c *Controller) runPhase(phase Phase) error {
	var (
		tasks  = phase.Tasks()
		wg     sync.WaitGroup
		result *multierror.Error
	)

	wg.Add(len(tasks))

	for number, task := range tasks {
		// Make the task number human friendly.
		number := number

		number++

		task := task

		go func() {
			defer wg.Done()

			start := time.Now()

			progress := fmt.Sprintf("%d/%d", number, len(tasks))

			log.Printf("task [%s]: starting", progress)
			defer log.Printf("task [%s]: done, %s", progress, time.Since(start))

			if err := c.runTask(task); err != nil {
				result = multierror.Append(result, err)
			}
		}()
	}

	wg.Wait()

	return result.ErrorOrNil()
}

func (c *Controller) runTask(t Task) error {
	if f := t.Func(c.Runtime.Platform().Mode()); f != nil {
		return f(c.Runtime)
	}

	return nil
}

func (c *Controller) phases(seq Sequence, data interface{}) ([]Phase, error) {
	var phases []Phase

	switch seq {
	case Boot:
		phases = c.Sequencer.Boot()
	case Initialize:
		phases = c.Sequencer.Initialize()
	case Shutdown:
		phases = c.Sequencer.Shutdown()
	case Reboot:
		phases = c.Sequencer.Reboot()
	case Upgrade:
		var (
			req *machine.UpgradeRequest
			ok  bool
		)

		if req, ok = data.(*machine.UpgradeRequest); !ok {
			return nil, ErrInvalidSequenceData
		}

		phases = c.Sequencer.Upgrade(req)
	case Reset:
		var (
			req *machine.ResetRequest
			ok  bool
		)

		if req, ok = data.(*machine.ResetRequest); !ok {
			return nil, ErrInvalidSequenceData
		}

		phases = c.Sequencer.Reset(req)
	}

	return phases, nil
}
