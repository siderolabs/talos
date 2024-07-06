// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"log"

	"github.com/cosi-project/runtime/pkg/controller"
)

// TaskSetupFunc defines the function that a task will execute for a specific runtime
// mode.
type TaskSetupFunc func(seq Sequence, data any) (TaskExecutionFunc, string)

// TaskExecutionFunc defines the function that a task will execute for a specific runtime
// mode.
type TaskExecutionFunc func(context.Context, *log.Logger, Runtime) error

// Phase represents a collection of tasks to be performed concurrently.
type Phase struct {
	Name      string
	Tasks     []TaskSetupFunc
	CheckFunc func() bool
}

// LockOptions represents the options for a controller.
type LockOptions struct {
	Takeover bool
}

// LockOption represents an option setter.
type LockOption func(o *LockOptions) error

// WithTakeover sets the take option to true.
func WithTakeover() LockOption {
	return func(o *LockOptions) error {
		o.Takeover = true

		return nil
	}
}

// DefaultControllerOptions returns the default controller options.
func DefaultControllerOptions() LockOptions {
	return LockOptions{}
}

// Controller represents the controller responsible for managing the execution
// of sequences.
type Controller interface {
	Runtime() Runtime
	Sequencer() Sequencer
	Run(context.Context, Sequence, any, ...LockOption) error
	V1Alpha2() V1Alpha2Controller
}

// V1Alpha2Controller provides glue into v2alpha1 controller runtime.
type V1Alpha2Controller interface {
	Run(context.Context, *Drainer) error
	DependencyGraph() (*controller.DependencyGraph, error)
}
