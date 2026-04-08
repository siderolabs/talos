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
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type task struct {
	args      []string
	state     runtimeres.TaskState
	startTime time.Time
	exitTime  time.Time
	err       error
}

type taskCompletion struct {
	ID       string
	err      error
	exitTime time.Time
}

// TasksController runs background tasks scheduled by other controllers.
type TasksController struct {
	Runtime    runtime.Runtime
	Tasks      map[string]task
	CompleteCh chan taskCompletion
}

// Name implements controller.Controller interface.
func (ctrl *TasksController) Name() string {
	return "runtime.TasksController"
}

// Inputs implements controller.Controller interface.
func (ctrl *TasksController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.TaskType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *TasksController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtimeres.TaskStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *TasksController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.Tasks == nil {
		ctrl.Tasks = make(map[string]task)
	}
	if ctrl.CompleteCh == nil {
		ctrl.CompleteCh = make(chan taskCompletion)
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case c := <-ctrl.CompleteCh:
			t := ctrl.Tasks[c.ID]
			logger.Warn("Task done", zap.Any("task", t))
			ctrl.Tasks[c.ID] = task{
				args:      t.args,
				state:     runtimeres.TaskStateCompleted,
				startTime: t.startTime,
				exitTime:  c.exitTime,
				err:       c.err,
			}
		case <-r.EventCh():
		}

		logger.Warn("task controller loop")

		cfg, err := safe.ReaderListAll[*runtimeres.Task](ctx, r)
		if err != nil && !state.IsNotFoundError(err) {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting scrub schedule: %w", err)
			}
		}

		for taskspec := range cfg.All() {
			taskspec := taskspec.TypedSpec()
			if _, ok := ctrl.Tasks[taskspec.ID]; !ok || ctrl.Tasks[taskspec.ID].state == runtimeres.TaskStateCreated {
				logger.Warn("creating a task or updating created and not ran task", zap.String("id", taskspec.ID))
				ctrl.Tasks[taskspec.ID] = task{
					args:  taskspec.Args,
					state: runtimeres.TaskStateCreated,
				}
			} else {
				logger.Warn("task updated while running", zap.String("task", taskspec.ID))
			}
		}

		for id := range ctrl.Tasks {
			_, err := safe.ReaderGetByID[*runtimeres.Task](ctx, r, id)
			if state.IsNotFoundError(err) {
				logger.Warn("Task removed, stopping and removing from list", zap.String("id", id))
				deschedule(id, ctrl)
			}
		}

		// if not currently running a task, find the first one to be ran
		for id := range ctrl.Tasks {
			task := ctrl.Tasks[id]
			if task.state == runtimeres.TaskStateCreated {
				// run the task
				logger.Warn("running task", zap.String("id", id))

				task.state = runtimeres.TaskStateRunning
				task.startTime = time.Now()
				ctrl.Tasks[id] = task

				runner := process.NewRunner(
					true, // debug
					&runner.Args{
						ID:          "task_runner",
						ProcessArgs: task.args,
					},
					runner.WithLoggingManager(ctrl.Runtime.Logging()),
					runner.WithEnv(environment.Get(ctrl.Runtime.Config())),
					runner.WithDroppedCapabilities(constants.XFSScrubDroppedCapabilities),
					runner.WithPriority(19),
					runner.WithIOPriority(runner.IoprioClassIdle, 7),
					runner.WithSchedulingPolicy(runner.SchedulingPolicyIdle),
				)

				go (func() {
					err := runner.Run(func(s events.ServiceState, msg string, args ...any) {}, func(serviceName string, pid int32, clearEntry bool) error { return nil })

					ctrl.CompleteCh <- taskCompletion{
						ID:       id,
						err:      err,
						exitTime: time.Now(),
					}
				})()

				break
			}
		}

		r.ResetRestartBackoff()

		r.StartTrackingOutputs()

		for id, t := range ctrl.Tasks {
			if err := safe.WriterModify(ctx, r, runtimeres.NewTaskStatus(id), func(status *runtimeres.TaskStatus) error {
				status.TypedSpec().ID = id
				status.TypedSpec().Duration = time.Since(t.startTime)

				if t.state == runtimeres.TaskStateCompleted {
					status.TypedSpec().Result = "Success"
					if t.err != nil {
						status.TypedSpec().Result = t.err.Error()
					}
				}

				status.TypedSpec().Start = t.startTime
				status.TypedSpec().TaskStatus = t.state

				return nil
			}); err != nil {
				return fmt.Errorf("error updating task status: %w", err)
			}
		}

		if err := safe.CleanupOutputs[*runtimeres.TaskStatus](ctx, r); err != nil {
			return err
		}
	}
}

func deschedule(id string, ctrl *TasksController) {
	delete(ctrl.Tasks, id)
}
