// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"
)

type task struct {
	id        string
	args      []string
	state     runtime.TaskState
	startTime time.Time
	duration  time.Duration
	exitCode  int
}

// TasksController runs background tasks scheduled by other controllers.
type TasksController struct {
	Tasks map[string]task
}

// Name implements controller.Controller interface.
func (ctrl *TasksController) Name() string {
	return "runtime.TasksController"
}

// Inputs implements controller.Controller interface.
func (ctrl *TasksController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.TaskType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *TasksController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.TaskStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *TasksController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-r.EventCh():
			cfg, err := safe.ReaderListAll[*runtime.Task](ctx, r)
			if err != nil && !state.IsNotFoundError(err) {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting scrub schedule: %w", err)
				}
			}

			for task := range cfg.All() {
			}
		}

		r.ResetRestartBackoff()
	}
}
