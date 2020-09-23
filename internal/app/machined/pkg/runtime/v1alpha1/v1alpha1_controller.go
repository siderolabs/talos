// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/acpi"
	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
)

// Controller represents the controller responsible for managing the execution
// of sequences.
type Controller struct {
	r *Runtime
	s *Sequencer

	semaphore int32
}

// NewController intializes and returns a controller.
func NewController(b []byte) (*Controller, error) {
	// Wait for USB storage in the case that the install disk is supplied over
	// USB. If we don't wait, there is the chance that we will fail to detect the
	// install disk.
	err := waitForUSBDelay()
	if err != nil {
		return nil, err
	}

	s, err := NewState()
	if err != nil {
		return nil, err
	}

	var cfg config.Provider

	if b != nil {
		cfg, err = configloader.NewFromBytes(b)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	// TODO: this should be streaming capacity and probably some constant
	e := NewEvents(1000, 10)

	l := logging.NewCircularBufferLoggingManager()

	ctlr := &Controller{
		r: NewRuntime(cfg, s, e, l),
		s: NewSequencer(),
	}

	return ctlr, nil
}

// Run executes all phases known to the controller in serial. `Controller`
// aborts immediately if any phase fails.
func (c *Controller) Run(seq runtime.Sequence, data interface{}, setters ...runtime.ControllerOption) error {
	// We must ensure that the runtime is configured since all sequences depend
	// on the runtime.
	if c.r == nil {
		return runtime.ErrUndefinedRuntime
	}

	opts := runtime.DefaultControllerOptions()

	for _, f := range setters {
		if err := f(&opts); err != nil {
			return err
		}
	}

	// Allow only one sequence to run at a time with the exception of bootstrap
	// and reset sequences.
	switch seq { //nolint: exhaustive
	case runtime.SequenceBootstrap, runtime.SequenceReset, runtime.SequenceRecover:
		// Do not attempt to lock.
	default:
		if opts.Force {
			break
		}

		if c.TryLock() {
			c.Runtime().Events().Publish(&machine.SequenceEvent{
				Sequence: seq.String(),
				Action:   machine.SequenceEvent_NOOP,
				Error: &common.Error{
					Code:    common.Code_LOCKED,
					Message: fmt.Sprintf("sequence not started: %s", runtime.ErrLocked.Error()),
				},
			})

			return runtime.ErrLocked
		}

		defer c.Unlock()
	}

	phases, err := c.phases(seq, data)
	if err != nil {
		return err
	}

	err = c.run(seq, phases, data)
	if err != nil {
		c.Runtime().Events().Publish(&machine.SequenceEvent{
			Sequence: seq.String(),
			Action:   machine.SequenceEvent_NOOP,
			Error: &common.Error{
				Code:    common.Code_FATAL,
				Message: fmt.Sprintf("sequence failed: %s", err.Error()),
			},
		})

		return err
	}

	return nil
}

// Runtime implements the controller interface.
func (c *Controller) Runtime() runtime.Runtime {
	return c.r
}

// Sequencer implements the controller interface.
func (c *Controller) Sequencer() runtime.Sequencer {
	return c.s
}

// ListenForEvents starts the event listener. The listener will trigger a
// shutdown in response to a SIGTERM signal and ACPI button/power event.
func (c *Controller) ListenForEvents() error {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGTERM)

	errCh := make(chan error, 2)

	go func() {
		<-sigs
		signal.Stop(sigs)

		log.Printf("shutdown via SIGTERM received")

		if err := c.Run(runtime.SequenceShutdown, nil); err != nil {
			log.Printf("shutdown failed: %v", err)
		}

		errCh <- nil
	}()

	if c.r.State().Platform().Mode() == runtime.ModeContainer {
		return nil
	}

	go func() {
		if err := acpi.StartACPIListener(); err != nil {
			errCh <- err

			return
		}

		log.Printf("shutdown via ACPI received")

		// TODO: The sequencer lock will prevent this. We need a way to force the
		// shutdown.
		if err := c.Run(runtime.SequenceShutdown, nil); err != nil {
			log.Printf("shutdown failed: %v", err)
		}

		errCh <- nil
	}()

	err := <-errCh

	return err
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

func (c *Controller) run(seq runtime.Sequence, phases []runtime.Phase, data interface{}) error {
	c.Runtime().Events().Publish(&machine.SequenceEvent{
		Sequence: seq.String(),
		Action:   machine.SequenceEvent_START,
	})

	defer c.Runtime().Events().Publish(&machine.SequenceEvent{
		Sequence: seq.String(),
		Action:   machine.SequenceEvent_STOP,
	})

	start := time.Now()

	var (
		number int
		phase  runtime.Phase
		err    error
	)

	log.Printf("%s sequence: %d phase(s)", seq.String(), len(phases))

	defer func() {
		if err != nil {
			log.Printf("%s sequence: failed", seq.String())
		}

		log.Printf("%s sequence: done: %s", seq.String(), time.Since(start))
	}()

	for number, phase = range phases {
		// Make the phase number human friendly.
		number++

		start := time.Now()

		progress := fmt.Sprintf("%d/%d", number, len(phases))

		log.Printf("phase %s (%s): %d tasks(s)", phase.Name, progress, len(phase.Tasks))

		if err = c.runPhase(phase, seq, data); err != nil {
			log.Printf("phase %s (%s): failed", phase.Name, progress)

			return fmt.Errorf("error running phase %d in %s sequence: %w", number, seq.String(), err)
		}

		log.Printf("phase %s (%s): done, %s", phase.Name, progress, time.Since(start))
	}

	return nil
}

func (c *Controller) runPhase(phase runtime.Phase, seq runtime.Sequence, data interface{}) error {
	c.Runtime().Events().Publish(&machine.PhaseEvent{
		Phase:  phase.Name,
		Action: machine.PhaseEvent_START,
	})

	defer c.Runtime().Events().Publish(&machine.PhaseEvent{
		Phase:  phase.Name,
		Action: machine.PhaseEvent_START,
	})

	var eg errgroup.Group

	for number, task := range phase.Tasks {
		// Make the task number human friendly.
		number := number

		number++

		task := task

		eg.Go(func() error {
			progress := fmt.Sprintf("%d/%d", number, len(phase.Tasks))

			if err := c.runTask(progress, task, seq, data); err != nil {
				return fmt.Errorf("task %s: failed, %w", progress, err)
			}

			return nil
		})
	}

	return eg.Wait()
}

func (c *Controller) runTask(progress string, f runtime.TaskSetupFunc, seq runtime.Sequence, data interface{}) error {
	task, taskName := f(seq, data)
	if task == nil {
		return nil
	}

	start := time.Now()

	c.Runtime().Events().Publish(&machine.TaskEvent{
		Task:   taskName,
		Action: machine.TaskEvent_START,
	})

	var err error

	log.Printf("task %s (%s): starting", taskName, progress)

	defer func() {
		if err != nil {
			log.Printf("task %s (%s): failed", taskName, progress)
		}

		log.Printf("task %s (%s): done, %s", taskName, progress, time.Since(start))
	}()

	defer c.Runtime().Events().Publish(&machine.TaskEvent{
		Task:   taskName,
		Action: machine.TaskEvent_STOP,
	})

	logger := &log.Logger{}

	var machinedLog io.WriteCloser

	machinedLog, err = c.Runtime().Logging().ServiceLog("machined").Writer()
	if err != nil {
		return err
	}

	defer machinedLog.Close() //nolint: errcheck

	if err = kmsg.SetupLogger(logger, fmt.Sprintf("[talos] task %s (%s):", taskName, progress), machinedLog); err != nil {
		return err
	}

	err = task(context.TODO(), logger, c.r)

	return err
}

// nolint: gocyclo
func (c *Controller) phases(seq runtime.Sequence, data interface{}) ([]runtime.Phase, error) {
	var phases []runtime.Phase

	switch seq {
	case runtime.SequenceApplyConfiguration:
		var (
			in *machine.ApplyConfigurationRequest
			ok bool
		)

		if in, ok = data.(*machine.ApplyConfigurationRequest); !ok {
			return nil, runtime.ErrInvalidSequenceData
		}

		phases = c.s.ApplyConfiguration(c.r, in)
	case runtime.SequenceBoot:
		phases = c.s.Boot(c.r)
	case runtime.SequenceBootstrap:
		phases = c.s.Bootstrap(c.r)
	case runtime.SequenceInitialize:
		phases = c.s.Initialize(c.r)
	case runtime.SequenceInstall:
		phases = c.s.Install(c.r)
	case runtime.SequenceShutdown:
		phases = c.s.Shutdown(c.r)
	case runtime.SequenceReboot:
		phases = c.s.Reboot(c.r)
	case runtime.SequenceRecover:
		var (
			in *machine.RecoverRequest
			ok bool
		)

		if in, ok = data.(*machine.RecoverRequest); !ok {
			return nil, runtime.ErrInvalidSequenceData
		}

		phases = c.s.Recover(c.r, in)
	case runtime.SequenceUpgrade:
		var (
			in *machine.UpgradeRequest
			ok bool
		)

		if in, ok = data.(*machine.UpgradeRequest); !ok {
			return nil, runtime.ErrInvalidSequenceData
		}

		phases = c.s.Upgrade(c.r, in)
	case runtime.SequenceReset:
		var (
			in *machine.ResetRequest
			ok bool
		)

		if in, ok = data.(*machine.ResetRequest); !ok {
			return nil, runtime.ErrInvalidSequenceData
		}

		phases = c.s.Reset(c.r, in)
	case runtime.SequenceNoop:
	default:
		return nil, fmt.Errorf("sequence not implemented: %q", seq)
	}

	return phases, nil
}

func waitForUSBDelay() (err error) {
	wait := true

	file := "/sys/module/usb_storage/parameters/delay_use"

	_, err = os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			wait = false
		} else {
			return err
		}
	}

	if wait {
		var b []byte

		b, err = ioutil.ReadFile(file)
		if err != nil {
			return err
		}

		val := strings.TrimSuffix(string(b), "\n")

		var i int

		i, err = strconv.Atoi(val)
		if err != nil {
			return err
		}

		log.Printf("waiting %d second(s) for USB storage", i)

		time.Sleep(time.Duration(i) * time.Second)
	}

	return nil
}
