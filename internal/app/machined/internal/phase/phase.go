/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package phase

import (
	"log"
	goruntime "runtime"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
)

// RuntimeArgs represents the set of arguments passed into a RuntimeFunc.
type RuntimeArgs struct {
	platform platform.Platform
	config   config.Configurator
}

// RuntimeFunc defines the function that a task must return. The function
// envelopes the task logic for a given runtime mode.
type RuntimeFunc func(*RuntimeArgs) error

// Task represents a task within a Phase.
type Task interface {
	RuntimeFunc(runtime.Mode) RuntimeFunc
}

// Phase represents a phase in the boot process.
type Phase struct {
	description string
	tasks       []Task
}

// Runner represents a management layer for phases.
type Runner struct {
	phases []*Phase
	args   *RuntimeArgs
}

// NewRunner initializes and returns a Runner.
func NewRunner(config config.Configurator) (*Runner, error) {
	platform, err := platform.NewPlatform()
	if err != nil {
		return nil, err
	}

	mode := platform.Mode()
	switch mode {
	case runtime.Metal:
		fallthrough
	case runtime.Cloud:
		// Setup logging to /dev/kmsg.
		if _, err = kmsg.Setup("[talos]"); err != nil {
			return nil, errors.Errorf("failed to setup logging to /dev/kmsg: %v", err)
		}
	}

	runner := &Runner{
		args: &RuntimeArgs{
			platform: platform,
			config:   config,
		},
	}

	return runner, nil
}

// Platform returns the platform.
func (r *RuntimeArgs) Platform() platform.Platform {
	return r.platform
}

// Config returns the config.
func (r *RuntimeArgs) Config() config.Configurator {
	return r.config
}

// Run executes sequentially all phases known to a Runner.
//
// If any phase fails, Runner aborts immediately.
func (r *Runner) Run() error {
	for _, phase := range r.phases {
		if err := r.runPhase(phase); err != nil {
			return errors.Wrapf(err, "error running phase %q", phase.description)
		}
	}

	return nil
}

// runPhase runs a phase by running all phase tasks concurrently.
func (r *Runner) runPhase(phase *Phase) error {
	errCh := make(chan error)

	start := time.Now()

	log.Printf("[phase]: %s", phase.description)

	for _, task := range phase.tasks {
		go r.runTask(task, errCh)
	}

	var result *multierror.Error

	for range phase.tasks {
		err := <-errCh
		if err != nil {
			log.Printf("[phase]: %s error running task: %s", phase.description, err)
		}

		result = multierror.Append(result, err)
	}

	log.Printf("[phase]: %s done, %s", phase.description, time.Since(start))

	return result.ErrorOrNil()
}

func (r *Runner) runTask(task Task, errCh chan<- error) {
	var err error

	defer func() {
		errCh <- err
	}()

	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 8192)
			n := goruntime.Stack(buf, false)
			err = errors.Errorf("panic recovered: %v\n%s", r, string(buf[:n]))
		}
	}()

	var f RuntimeFunc
	if f = task.RuntimeFunc(r.args.Platform().Mode()); f == nil {
		// A task is not defined for this runtime mode.
		return
	}

	err = f(r.args)
}

// Add adds a phase to a Runner.
func (r *Runner) Add(phase ...*Phase) {
	r.phases = append(r.phases, phase...)
}

// NewPhase initializes and returns a Phase.
func NewPhase(description string, tasks ...Task) *Phase {
	tasks = append([]Task{}, tasks...)

	return &Phase{
		description: description,
		tasks:       tasks,
	}
}
