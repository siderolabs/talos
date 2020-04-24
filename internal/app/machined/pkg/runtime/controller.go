// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"log"
)

// TaskSetupFunc defines the function that a task will execute for a specific runtime
// mode.
type TaskSetupFunc func(seq Sequence, data interface{}) TaskExecutionFunc

// TaskExecutionFunc defines the function that a task will execute for a specific runtime
// mode.
type TaskExecutionFunc func(context.Context, *log.Logger, Runtime) error

// Phase represents a collection of tasks to be performed concurrently.
type Phase []TaskSetupFunc

// Controller represents the controller responsible for managing the execution
// of sequences.
type Controller interface {
	Runtime() Runtime
	Sequencer() Sequencer
	Run(Sequence, interface{}) error
}
