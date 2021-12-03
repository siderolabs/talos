// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/acpi"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha2"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
)

// Controller represents the controller responsible for managing the execution
// of sequences.
type Controller struct {
	r  *Runtime
	s  *Sequencer
	v2 *v1alpha2.Controller

	semaphore int32
	cancelCtx context.CancelFunc
	ctxMutex  sync.Mutex
}

// NewController intializes and returns a controller.
func NewController() (*Controller, error) {
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

	// TODO: this should be streaming capacity and probably some constant
	e := NewEvents(1000, 10)

	l := logging.NewCircularBufferLoggingManager(log.New(os.Stdout, "machined fallback logger: ", log.Flags()))

	ctlr := &Controller{
		r: NewRuntime(nil, s, e, l),
		s: NewSequencer(),
	}

	ctlr.v2, err = v1alpha2.NewController(ctlr.r)
	if err != nil {
		return nil, err
	}

	return ctlr, nil
}

// Run executes all phases known to the controller in serial. `Controller`
// aborts immediately if any phase fails.
//nolint:gocyclo
func (c *Controller) Run(ctx context.Context, seq runtime.Sequence, data interface{}, setters ...runtime.ControllerOption) error {
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
	switch seq { //nolint:exhaustive
	case runtime.SequenceBootstrap, runtime.SequenceReset:
		// Do not attempt to lock.
	default:
		if opts.Force {
			break
		}

		if opts.Takeover {
			c.ctxMutex.Lock()
			if c.cancelCtx != nil {
				c.cancelCtx()
			}

			c.ctxMutex.Unlock()

			err := retry.Constant(time.Minute*1, retry.WithUnits(time.Millisecond*100)).RetryWithContext(ctx, func(ctx context.Context) error {
				if c.TryLock() {
					return retry.ExpectedError(fmt.Errorf("failed to acquire lock"))
				}

				return nil
			})
			if err != nil {
				return err
			}
		} else if c.TryLock() {
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

		c.ctxMutex.Lock()
		ctx, c.cancelCtx = context.WithCancel(ctx)
		c.ctxMutex.Unlock()

		defer func() {
			c.ctxMutex.Lock()
			c.cancelCtx = nil
			c.ctxMutex.Unlock()
		}()
	}

	phases, err := c.phases(seq, data)
	if err != nil {
		return err
	}

	err = c.run(ctx, seq, phases, data)
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

// V1Alpha2 implements the controller interface.
func (c *Controller) V1Alpha2() runtime.V1Alpha2Controller {
	return c.v2
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
func (c *Controller) ListenForEvents(ctx context.Context) error {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGTERM)

	errCh := make(chan error, 2)

	go func() {
		<-sigs
		signal.Stop(sigs)

		log.Printf("shutdown via SIGTERM received")

		if err := c.Run(ctx, runtime.SequenceShutdown, nil, runtime.WithTakeover()); err != nil {
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

		if err := c.Run(ctx, runtime.SequenceShutdown, nil, runtime.WithTakeover()); err != nil {
			log.Printf("failed to run shutdown sequence: %s", err)
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

func (c *Controller) run(ctx context.Context, seq runtime.Sequence, phases []runtime.Phase, data interface{}) error {
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
			if !runtime.IsRebootError(err) {
				log.Printf("%s sequence: failed", seq.String())
			}
		} else {
			log.Printf("%s sequence: done: %s", seq.String(), time.Since(start))
		}
	}()

	for number, phase = range phases {
		// Make the phase number human friendly.
		number++

		start := time.Now()

		progress := fmt.Sprintf("%d/%d", number, len(phases))

		log.Printf("phase %s (%s): %d tasks(s)", phase.Name, progress, len(phase.Tasks))

		if err = c.runPhase(ctx, phase, seq, data); err != nil {
			if !runtime.IsRebootError(err) {
				log.Printf("phase %s (%s): failed", phase.Name, progress)
			}

			return fmt.Errorf("error running phase %d in %s sequence: %w", number, seq.String(), err)
		}

		log.Printf("phase %s (%s): done, %s", phase.Name, progress, time.Since(start))

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

func (c *Controller) runPhase(ctx context.Context, phase runtime.Phase, seq runtime.Sequence, data interface{}) error {
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

			if err := c.runTask(ctx, progress, task, seq, data); err != nil {
				return fmt.Errorf("task %s: failed, %w", progress, err)
			}

			return nil
		})
	}

	return eg.Wait()
}

func (c *Controller) runTask(ctx context.Context, progress string, f runtime.TaskSetupFunc, seq runtime.Sequence, data interface{}) error {
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
			if !runtime.IsRebootError(err) {
				log.Printf("task %s (%s): failed: %s", taskName, progress, err)
			}
		} else {
			log.Printf("task %s (%s): done, %s", taskName, progress, time.Since(start))
		}
	}()

	defer c.Runtime().Events().Publish(&machine.TaskEvent{
		Task:   taskName,
		Action: machine.TaskEvent_STOP,
	})

	logger := log.New(log.Writer(), fmt.Sprintf("[talos] task %s (%s): ", taskName, progress), log.Flags())

	err = task(ctx, logger, c.r)

	return err
}

//nolint:gocyclo
func (c *Controller) phases(seq runtime.Sequence, data interface{}) ([]runtime.Phase, error) {
	var phases []runtime.Phase

	switch seq {
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
	case runtime.SequenceUpgrade:
		var (
			in *machine.UpgradeRequest
			ok bool
		)

		if in, ok = data.(*machine.UpgradeRequest); !ok {
			return nil, runtime.ErrInvalidSequenceData
		}

		phases = c.s.Upgrade(c.r, in)
	case runtime.SequenceStageUpgrade:
		var (
			in *machine.UpgradeRequest
			ok bool
		)

		if in, ok = data.(*machine.UpgradeRequest); !ok {
			return nil, runtime.ErrInvalidSequenceData
		}

		phases = c.s.StageUpgrade(c.r, in)
	case runtime.SequenceReset:
		var (
			in runtime.ResetOptions
			ok bool
		)

		if in, ok = data.(runtime.ResetOptions); !ok {
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
