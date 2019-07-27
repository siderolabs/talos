/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package phase

import (
	"log"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform/container"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Runner represents a management layer for phases.
type Runner struct {
	platform platform.Platform
	phases   []*Phase
	mode     runtime.Mode
	data     *userdata.UserData
}

// NewRunner initializes and returns a Runner.
func NewRunner(data *userdata.UserData) (*Runner, error) {
	platform, err := platform.NewPlatform()
	if err != nil {
		return nil, err
	}

	mode := runtime.Standard
	switch platform.(type) {
	case *container.Container:
		mode = runtime.Container
	default:
		// Setup logging to /dev/kmsg.
		if _, err = kmsg.Setup("[talos]"); err != nil {
			return nil, errors.Errorf("failed to setup logging to /dev/kmsg: %v", err)
		}
	}

	return &Runner{
		platform: platform,
		mode:     mode,
		data:     data,
	}, nil
}

// Run executes all phases known to a Runner.
func (r *Runner) Run() error {
	for _, phase := range r.phases {
		var (
			result *multierror.Error
			wg     sync.WaitGroup
		)
		wg.Add(len(phase.tasks))
		log.Printf("[phase]: %s", phase.name)
		go func(p *Phase) {
			for _, task := range p.tasks {
				defer wg.Done()
				var f RuntimeFunc
				if f = task.RuntimeFunc(r.mode); f == nil {
					// A task is not defined for this runtime mode.
					continue
				}
				if err := f(r.platform, r.data); err != nil {
					result = multierror.Append(result, err)
				}
			}
		}(phase)
		wg.Wait()
		if result != nil {
			return result.ErrorOrNil()
		}
	}

	return nil
}

// Add adds a phase to a Runner.
func (r *Runner) Add(phase ...*Phase) {
	r.phases = append(r.phases, phase...)
}

// Phase represents a phase in the boot process.
type Phase struct {
	name  string
	tasks []Task
}

// NewPhase initializes and returns a Phase.
func NewPhase(name string, tasks ...Task) *Phase {
	tasks = append([]Task{}, tasks...)
	return &Phase{
		name:  name,
		tasks: tasks,
	}
}

// RuntimeFunc defines the function that a task must return. The function
// envelopes the task logic for a given runtim mode.
type RuntimeFunc func(platform.Platform, *userdata.UserData) error

// Task represents a task within a Phase.
type Task interface {
	RuntimeFunc(runtime.Mode) RuntimeFunc
}
