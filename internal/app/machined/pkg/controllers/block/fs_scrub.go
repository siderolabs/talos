// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"math/rand/v2"
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
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VolumeStatus -[ScheduleController]-> ScrubSchedule -[ScrubRunController]-> Task -[TaskController]-> Result
//                                                          |
//                                                          |
//                                                          v
//                                                          ScrubSummaryStatus

type scrubSchedule struct {
	mountpoint string
	period     time.Duration
	timer      *time.Timer
}

type scrubStatus struct {
	id         string
	mountpoint string
	period     time.Duration
	time       time.Time
	duration   time.Duration
	result     error
}

// FSScrubController watches v1alpha1.Config and schedules filesystem online check tasks.
type FSScrubController struct {
	Runtime  runtime.Runtime
	schedule map[string]scrubSchedule
	status   map[string]scrubStatus
	// When a mountpoint is scheduled to be scrubbed, its path is sent to this channel to be processed in the Run function.
	c chan string
}

// Name implements controller.Controller interface.
func (ctrl *FSScrubController) Name() string {
	return "block.FSScrubController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FSScrubController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.FSScrubScheduleType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.MountStatusType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *FSScrubController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.FSScrubStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: runtimeres.TaskType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *FSScrubController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	stopTimers := func() {
		for _, task := range ctrl.schedule {
			if task.timer != nil {
				task.timer.Stop()
			}
		}
	}

	defer stopTimers()

	ctrl.schedule = make(map[string]scrubSchedule)
	ctrl.status = make(map[string]scrubStatus)
	ctrl.c = make(chan string, 5)

	for {
		select {
		case <-ctx.Done():
			return nil
		case mountpoint := <-ctrl.c:
			if err := ctrl.runScrub(ctx, mountpoint, []string{}, r, logger); err != nil {
				logger.Error("error running filesystem scrub", zap.Error(err))
			}
		case <-r.EventCh():
			err := ctrl.updateSchedule(ctx, r, logger)
			if err != nil {
				return err
			}
		}

		if err := ctrl.reportStatus(ctx, r); err != nil {
			return err
		}
	}
}

func (ctrl *FSScrubController) reportStatus(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	presentStatuses, err := safe.ReaderListAll[*block.FSScrubStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting existing FS scrub statuses: %w", err)
	}

	for entry := range presentStatuses.All() {
		if _, ok := ctrl.status[entry.TypedSpec().Mountpoint]; !ok {
			if err := r.Destroy(ctx, block.NewFSScrubStatus(entry.Metadata().ID()).Metadata()); err != nil {
				return fmt.Errorf("error destroying old FS scrub status: %w", err)
			}
		}
	}

	for _, entry := range ctrl.status {
		if err := safe.WriterModify(ctx, r, block.NewFSScrubStatus(entry.id), func(status *block.FSScrubStatus) error {
			status.TypedSpec().Mountpoint = entry.mountpoint
			status.TypedSpec().Period = entry.period
			status.TypedSpec().Time = entry.time
			status.TypedSpec().Duration = entry.duration

			if entry.result != nil {
				status.TypedSpec().Status = entry.result.Error()
			} else {
				status.TypedSpec().Status = "success"
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating filesystem scrub status: %w", err)
		}
	}

	if err := safe.CleanupOutputs[*block.FSScrubStatus](ctx, r); err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo,cyclop
func (ctrl *FSScrubController) updateSchedule(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	cfg, err := safe.ReaderListAll[*block.FSScrubSchedule](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting scrub schedule: %w", err)
		}
	}

	// Cancel the timers once the schedule is removed.
	for mountpoint := range ctrl.schedule {
		isScheduled := false

		for item := range cfg.All() {
			scheduledTask := item.TypedSpec()

			if scheduledTask.Mountpoint == mountpoint && scheduledTask.Period == ctrl.schedule[mountpoint].period {
				isScheduled = true

				break
			}
		}

		if !isScheduled {
			ctrl.cancelScrub(mountpoint)
		}
	}

	for item := range cfg.All() {
		scheduledTask := item.TypedSpec()
		mountpoint := scheduledTask.Mountpoint
		period := scheduledTask.Period

		_, ok := ctrl.schedule[mountpoint]

		if ok {
			ctrl.schedule[mountpoint].timer.Stop()
		}

		firstTimeout := time.Until(scheduledTask.StartTime)
		if firstTimeout < 0 {
			logger.Warn("scrub schedule start time is in the past, using random timeout", zap.String("mountpoint", mountpoint))
			firstTimeout = time.Duration(rand.Int64N(int64(period.Seconds()))) * time.Second
		}

		// When scheduling the first scrub, we use a random time to avoid all scrubs running in a row.
		// After the first scrub, we use the period defined in the config.
		cb := func() {
			ctrl.c <- mountpoint
			ctrl.schedule[mountpoint].timer.Reset(ctrl.schedule[mountpoint].period)
		}

		ctrl.schedule[mountpoint] = scrubSchedule{
			mountpoint: mountpoint,
			period:     period,
			timer:      time.AfterFunc(firstTimeout, cb),
		}

		ctrl.status[mountpoint] = scrubStatus{
			id:         item.Metadata().ID(),
			mountpoint: mountpoint,
			period:     period,
			time:       time.Now().Add(firstTimeout),
			duration:   0,
			result:     fmt.Errorf("scheduled"),
		}
	}

	return err
}

func (ctrl *FSScrubController) cancelScrub(mountpoint string) {
	ctrl.schedule[mountpoint].timer.Stop()

	// TODO: stop the process if it's running

	delete(ctrl.schedule, mountpoint)
	delete(ctrl.status, mountpoint)
}

func (ctrl *FSScrubController) runScrub(ctx context.Context, mountpoint string, opts []string, r controller.Runtime, logger *zap.Logger) error {
	args := []string{"/usr/sbin/xfs_scrub", "-T", "-v"}
	args = append(args, opts...)
	args = append(args, mountpoint)

	runner := process.NewRunner(
		true,
		&runner.Args{
			ID:          "fs_scrub",
			ProcessArgs: args,
		},
		runner.WithLoggingManager(ctrl.Runtime.Logging()),
		runner.WithEnv(environment.Get(ctrl.Runtime.Config())),
		runner.WithDroppedCapabilities(constants.XFSScrubDroppedCapabilities),
		runner.WithPriority(19),
		runner.WithIOPriority(runner.IoprioClassIdle, 7),
		runner.WithSchedulingPolicy(runner.SchedulingPolicyIdle),
	)

	start := time.Now()

	task := runtimeres.NewTask()
	if err := safe.WriterModify(ctx, r, task, func(status *runtimeres.Task) error {
		status.TypedSpec().Args = args
		status.TypedSpec().TaskName = "fs_scrub"
		status.TypedSpec().ID = ctrl.status[mountpoint].id

		return nil
	}); err != nil {
		return fmt.Errorf("error updating filesystem task scredule: %w", err)
	}

	mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting mount statuses to obtain finalizers: %w", err)
	}

	var mountStatus *block.MountStatus
	for entry := range mountStatuses.All() {
		if entry.TypedSpec().Target == mountpoint {
			mountStatus = entry
			break
		}
	}

	if mountStatus != nil {
		if err := r.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error adding finalizer: %w", err)
		}
		fmt.Println("added finalizer to mount status", zap.String("mountpoint", mountpoint), zap.String("finalizer", ctrl.Name()))
	}

	err = runner.Run(func(s events.ServiceState, msg string, args ...any) {})
	// delete the task
	r.Destroy(ctx, task.Metadata())

	if mountStatus != nil {
		if err := r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer: %w", err)
		}
		fmt.Println("removed finalizer from mount status", zap.String("mountpoint", mountpoint), zap.String("finalizer", ctrl.Name()))
	}

	ctrl.status[mountpoint] = scrubStatus{
		id:         ctrl.status[mountpoint].id,
		mountpoint: mountpoint,
		period:     ctrl.schedule[mountpoint].period,
		time:       start,
		duration:   time.Since(start),
		result:     err,
	}

	return err
}
