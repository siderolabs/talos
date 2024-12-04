// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
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
	args         []string
	selinuxLabel string
	owner        string
	state        runtimeres.TaskState
	startTime    time.Time
	exitTime     time.Time
	err          error
	stop         chan any
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

	// If not nil, use this function to create runner, used for mocks
	NewRunner func(rt runtime.Runtime, args *runner.Args, selinuxLabel string) runner.Runner
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
			if t.state != runtimeres.TaskStateRunning {
				continue
			}

			logger.Info("Task done", zap.Any("task", t))

			if err := ctrl.removeFinalizer(ctx, r, c.ID); err != nil {
				return fmt.Errorf("failed to remove finalizer for task %q: %w", c.ID, err)
			}

			// The task resource has been removed, remove from the map
			if t.stop == nil {
				delete(ctrl.Tasks, c.ID)
			} else {
				ctrl.Tasks[c.ID] = task{
					args:         t.args,
					selinuxLabel: t.selinuxLabel,
					owner:        t.owner,
					state:        runtimeres.TaskStateCompleted,
					startTime:    t.startTime,
					exitTime:     c.exitTime,
					err:          c.err,
					stop:         make(chan any),
				}
			}
		case <-r.EventCh():
		}

		logger.Warn("task controller loop")

		cfg, err := safe.ReaderListAll[*runtimeres.Task](ctx, r)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting tasks: %w", err)
		}

		for taskspec := range cfg.All() {
			// Do not react to changes in tearing down resources
			if taskspec.Metadata().Phase() == resource.PhaseTearingDown {
				continue
			}

			spec := taskspec.TypedSpec()
			if _, ok := ctrl.Tasks[spec.ID]; !ok || ctrl.Tasks[spec.ID].state == runtimeres.TaskStateCreated {
				logger.Warn("creating a task or updating created and not ran task", zap.String("id", spec.ID))
				ctrl.Tasks[spec.ID] = task{
					args:         spec.Args,
					selinuxLabel: spec.SelinuxLabel,
					owner:        spec.Owner,
					state:        runtimeres.TaskStateCreated,
					startTime:    time.Now(),
					exitTime:     time.Now(),
					stop:         make(chan any),
				}
			} else {
				logger.Warn("task updated while running", zap.String("task", spec.ID))
			}
		}

		for id := range ctrl.Tasks {
			res, err := safe.ReaderGetByID[*runtimeres.Task](ctx, r, id)

			removing := state.IsNotFoundError(err) ||
				(err == nil && res.Metadata().Phase() == resource.PhaseTearingDown)

			if removing {
				logger.Warn("Task removed or tearing down, stopping", zap.String("id", id))
				t := ctrl.Tasks[id]

				if t.stop != nil {
					close(t.stop)
				}

				if ctrl.Tasks[id].state != runtimeres.TaskStateCompleted {
					ctrl.Tasks[id] = task{
						args:         t.args,
						selinuxLabel: t.selinuxLabel,
						owner:        t.owner,
						state:        t.state,
						startTime:    t.startTime,
						exitTime:     t.exitTime,
						err:          t.err,
						stop:         nil,
					}
				} else {
					delete(ctrl.Tasks, id)
				}

				// After the task has stopped, remove it from the map, and thus remove status
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
				task.exitTime = task.startTime
				ctrl.Tasks[id] = task

				if err = ctrl.addFinalizer(ctx, r, id); err != nil {
					return fmt.Errorf("error adding a finalizer: %w", err)
				}

				taskRunner := ctrl.newRunner(task.args, task.selinuxLabel)

				go (func() {
					err := taskRunner.Run(func(s events.ServiceState, msg string, args ...any) {}, func(serviceName string, pid int32, clearEntry bool) error { return nil })

					ctrl.CompleteCh <- taskCompletion{
						ID:       id,
						err:      err,
						exitTime: time.Now(),
					}
				})()

				go (func() {
					<-task.stop

					fmt.Println("STOPPING")

					if err := taskRunner.Stop(); err != nil {
						logger.Error("Failed to stop task", zap.Error(err))
					}

					ctrl.CompleteCh <- taskCompletion{
						ID:       id,
						err:      fmt.Errorf("Canceled"),
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
				status.TypedSpec().Owner = t.owner
				status.TypedSpec().Duration = t.exitTime.Sub(t.startTime)

				if t.state == runtimeres.TaskStateCompleted {
					status.TypedSpec().Result = "Success"
					if t.err != nil {
						status.TypedSpec().Result = t.err.Error()
					}
				}

				status.TypedSpec().Start = t.startTime
				status.TypedSpec().TaskState = t.state

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

// newRunner builds a runner.Runner for a single task invocation, using mock if provided.
func (ctrl *TasksController) newRunner(args []string, selinuxLabel string) runner.Runner {
	rargs := &runner.Args{
		ID:          "task_runner",
		ProcessArgs: args,
	}

	if ctrl.NewRunner != nil {
		return ctrl.NewRunner(ctrl.Runtime, rargs, selinuxLabel)
	}

	return process.NewRunner(
		false,
		rargs,
		runner.WithLoggingManager(ctrl.Runtime.Logging()),
		runner.WithEnv(environment.Get(ctrl.Runtime.Config())),
		// TODO: configure via the resource?
		runner.WithDroppedCapabilities(constants.XFSScrubDroppedCapabilities),
		runner.WithPriority(19),
		runner.WithIOPriority(runner.IoprioClassIdle, 7),
		runner.WithSchedulingPolicy(runner.SchedulingPolicyIdle),
		runner.WithSelinuxLabel(selinuxLabel),
	)
}

func (ctrl *TasksController) addFinalizer(ctx context.Context, r controller.Runtime, id string) error {
	t, err := safe.ReaderGetByID[*runtimeres.Task](ctx, r, id)
	if err != nil {
		return err
	}

	if t.Metadata().Finalizers().Has(ctrl.Name()) {
		return fmt.Errorf("%s already has a %s finalizer", id, ctrl.Name())
	}

	if err = r.AddFinalizer(ctx, t.Metadata(), ctrl.Name()); err != nil {
		return err
	}

	return nil
}

func (ctrl *TasksController) removeFinalizer(ctx context.Context, r controller.Runtime, id string) error {
	t, err := safe.ReaderGetByID[*runtimeres.Task](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if !t.Metadata().Finalizers().Has(ctrl.Name()) {
		return fmt.Errorf("%s already has no %s finalizer", id, ctrl.Name())
	}

	if err = r.RemoveFinalizer(ctx, t.Metadata(), ctrl.Name()); err != nil {
		return err
	}

	return nil
}
