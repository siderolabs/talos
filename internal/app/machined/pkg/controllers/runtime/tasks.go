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
	args      []string
	state     runtime.TaskState
	startTime time.Time
	duration  time.Duration
	exitCode  int
}

// TasksController runs background tasks scheduled by other controllers.
type TasksController struct {
	Tasks       map[string]task
	RunningTask string
	CompleteCh  <-chan struct{}
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
	ctrl.Tasks = make(map[string]task)

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-r.EventCh():
		case <-ctrl.CompleteCh:
			cfg, err := safe.ReaderListAll[*runtime.Task](ctx, r)
			if err != nil && !state.IsNotFoundError(err) {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting scrub schedule: %w", err)
				}
			}

			for taskspec := range cfg.All() {
				taskspec := taskspec.TypedSpec()
				if _, ok := ctrl.Tasks[taskspec.ID]; !ok || ctrl.Tasks[taskspec.ID].state == runtime.TaskStateCreated {
					fmt.Println("creating a task or updating created and not ran task", taskspec.ID)
					ctrl.Tasks[taskspec.ID] = task{
						args:      taskspec.Args,
						state:     runtime.TaskStateCreated,
						startTime: time.UnixMicro(0),
						duration:  0,
						exitCode:  0,
					}
				} else {
					logger.Warn("task updated while running", zap.String("task", taskspec.ID))
				}
			}

			for id := range ctrl.Tasks {
				_, err := safe.ReaderGetByID[*runtime.Task](ctx, r, id)
				if state.IsNotFoundError(err) {
					deschedule(id, ctrl)
				}
			}

			if ctrl.RunningTask != "" {
				// check status and report
				continue
			}

			// if not currently running a task, find the first one to be ran
			for id := range ctrl.Tasks {
				task := ctrl.Tasks[id]
				if task.state == runtime.TaskStateCreated {
					// run the task
					fmt.Println("running task", id)
					task.state = runtime.TaskStateRunning
					task.startTime = time.Now()
					ctrl.Tasks[id] = task
					ctrl.RunningTask = id
					break
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func deschedule(id string, ctrl *TasksController) {
	fmt.Println("Task removed, stopping and removing from list", id)
	delete(ctrl.Tasks, id)
}
