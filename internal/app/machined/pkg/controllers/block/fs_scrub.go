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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VolumeStatus -[ScheduleController]-> ScrubSchedule -[ScrubRunController]-> Task -[TaskController]-> Result
//                                                          |          ^                                 |
//                                                          |          \---------------------------------/
//                                                          v
//                                                          ScrubSummaryStatus

type scrubSchedule struct {
	mountpoint string
	period     time.Duration
	// stop cancels the goroutine driving this schedule's timer.
	stop func()
}

type scrubTask struct {
	Args       []string
	Destroying bool
}

type scrubStatus struct {
	id         string
	mountpoint string
	period     time.Duration
	time       time.Time
	duration   time.Duration
	result     string
}

// FSScrubController watches v1alpha1.Config and schedules filesystem online check tasks.
type FSScrubController struct {
	Runtime  runtime.Runtime
	schedule map[string]scrubSchedule
	tasks    map[string]scrubTask
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
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.TaskType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.TaskStatusType,
			Kind:      controller.InputWeak,
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

func (ctrl *FSScrubController) init() {
	if ctrl.schedule == nil {
		ctrl.schedule = make(map[string]scrubSchedule)
	}

	if ctrl.status == nil {
		ctrl.status = make(map[string]scrubStatus)
	}

	if ctrl.tasks == nil {
		ctrl.tasks = make(map[string]scrubTask)
	}

	if ctrl.c == nil {
		ctrl.c = make(chan string, 5)
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *FSScrubController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.init()

	defer func() {
		for _, sched := range ctrl.schedule {
			if sched.stop != nil {
				sched.stop()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case mountpoint := <-ctrl.c:
			if err := ctrl.createScrubTask(ctx, logger, mountpoint, []string{}, r); err != nil {
				logger.Error("error running filesystem scrub", zap.Error(err))
			}
		case <-r.EventCh():
			err := ctrl.processStatuses(ctx, r)
			if err != nil {
				return err
			}

			err = ctrl.updateSchedule(ctx, r, logger)
			if err != nil {
				return err
			}

			err = ctrl.handleTeardown(ctx, r)
			if err != nil {
				return err
			}
		}

		if err := ctrl.outputTasks(ctx, r); err != nil {
			return err
		}

		if err := ctrl.reportStatus(ctx, r); err != nil {
			return err
		}
	}
}

// processStatuses reads TaskStatus resources from the task controller and handles
// results of the FS scrubbing tasks.
//
//nolint:gocyclo
func (ctrl *FSScrubController) processStatuses(ctx context.Context, r controller.Runtime) error {
	taskStatuses, err := safe.ReaderListAll[*runtimeres.TaskStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting task statuses: %w", err)
	}

	for s := range taskStatuses.All() {
		if s.TypedSpec().Owner != ctrl.Name() || s.TypedSpec().TaskState != runtimeres.TaskStateCompleted {
			continue
		}

		mountpoint := s.TypedSpec().ID

		if _, ok := ctrl.tasks[mountpoint]; !ok {
			continue
		}

		ctrl.tasks[mountpoint] = scrubTask{
			Args:       ctrl.tasks[mountpoint].Args,
			Destroying: true,
		}

		mountStatus, err := ctrl.getMountStatus(ctx, r, mountpoint)
		if err != nil {
			return err
		}

		if mountStatus != nil && mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			if err := r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error removing finalizer: %w", err)
			}
		}

		st, ok := ctrl.status[mountpoint]
		if !ok {
			// Task has been descheduled before it completed
			// Do not report status
			continue
		}

		ctrl.status[mountpoint] = scrubStatus{
			id:         st.id,
			mountpoint: mountpoint,
			// Only deleted after status
			period:   ctrl.schedule[mountpoint].period,
			time:     s.TypedSpec().Start,
			duration: s.TypedSpec().Duration,
			result:   s.TypedSpec().Result,
		}
	}

	return nil
}

//nolint:gocyclo
func (ctrl *FSScrubController) outputTasks(ctx context.Context, r controller.Runtime) error {
	currentTasks, err := safe.ReaderListAll[*runtimeres.Task](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting existing FS scrub statuses: %w", err)
	}

	for entry := range currentTasks.All() {
		id := entry.TypedSpec().ID
		if t, ok := ctrl.tasks[id]; ok && t.Destroying {
			metadata := entry.Metadata()

			okToDestroy, err := r.Teardown(ctx, metadata)
			if err != nil {
				return fmt.Errorf("error destroying old FS scrub tasks: %w", err)
			}

			// FIXME: this is sort of double-signaled, because TasksController
			// both holds a finalizer and reports result, and we use finalizer to
			// remove the task, but wait for result to drop our own finalizer
			if okToDestroy {
				if err := r.Destroy(ctx, metadata); err != nil {
					return fmt.Errorf("error destroying old FS scrub tasks: %w", err)
				}

				delete(ctrl.tasks, id)
			}
		}
	}

	for mountpoint, entry := range ctrl.tasks {
		if entry.Destroying {
			continue
		}

		if err := safe.WriterModify(ctx, r, runtimeres.NewTask(mountpoint), func(status *runtimeres.Task) error {
			status.TypedSpec().ID = mountpoint
			status.TypedSpec().Args = entry.Args
			status.TypedSpec().Owner = ctrl.Name()
			status.TypedSpec().SelinuxLabel = constants.SelinuxLabelMachined

			return nil
		}); err != nil {
			return fmt.Errorf("error updating task: %w", err)
		}
	}

	return nil
}

func (ctrl *FSScrubController) reportStatus(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	for _, entry := range ctrl.status {
		if err := safe.WriterModify(ctx, r, block.NewFSScrubStatus(entry.id), func(status *block.FSScrubStatus) error {
			status.TypedSpec().Mountpoint = entry.mountpoint
			status.TypedSpec().Period = entry.period
			status.TypedSpec().Time = entry.time
			status.TypedSpec().Duration = entry.duration

			if entry.result != "" {
				status.TypedSpec().Status = entry.result
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
		return fmt.Errorf("error getting scrub schedule: %w", err)
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
			if ctrl.schedule[mountpoint].period == period {
				continue
			}

			if stop := ctrl.schedule[mountpoint].stop; stop != nil {
				stop()
			}
		}

		firstTimeout := time.Until(scheduledTask.StartTime)
		if firstTimeout < 0 {
			logger.Warn("scrub schedule start time is in the past, using random timeout", zap.String("mountpoint", mountpoint))

			firstTimeout = time.Duration(rand.Int64N(int64(period.Seconds()))) * time.Second
		}

		// Drive the schedule from a dedicated goroutine rather than a
		// self-resetting time.AfterFunc callback: the callback would read the
		// timer variable that the creating goroutine writes after AfterFunc
		// returns, which is an unsynchronized access (data race). Here the timer
		// is fully owned by the goroutine, and cancellation goes through the
		// context derived from Run's ctx.
		//
		// When scheduling the first scrub, we use a random time to avoid all
		// scrubs running in a row. After the first scrub, we use the period
		// defined in the config.
		scrubCtx, cancel := context.WithCancel(ctx)
		timer := time.NewTimer(firstTimeout)

		go func() {
			defer timer.Stop()

			for {
				select {
				case <-scrubCtx.Done():
					return
				case <-timer.C:
					timer.Reset(period)

					select {
					case ctrl.c <- mountpoint:
					case <-scrubCtx.Done():
						return
					default:
						logger.Warn("scrub trigger channel is full, skipping scheduled scrub trigger", zap.String("mountpoint", mountpoint))
					}
				}
			}
		}()

		ctrl.schedule[mountpoint] = scrubSchedule{
			mountpoint: mountpoint,
			period:     period,
			stop:       cancel,
		}

		logger.Warn("scrub schedule", zap.String("mountpoint", mountpoint), zap.Duration("period", period), zap.Time("startTime", scheduledTask.StartTime))

		ctrl.status[mountpoint] = scrubStatus{
			id:         item.Metadata().ID(),
			mountpoint: mountpoint,
			period:     period,
			time:       time.Now().Add(firstTimeout),
			duration:   0,
			result:     "scheduled",
		}
	}

	return err
}

// handleTeardown stops scrubbing when a volume enters the teardown phase and
// releases the finalizer once the scrub task is gone so the mount can finish
// unmounting.
func (ctrl *FSScrubController) handleTeardown(ctx context.Context, r controller.Runtime) error {
	mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error monitoring mount status teardowns: %w", err)
	}

	for entry := range mountStatuses.All() {
		if entry.Metadata().Phase() != resource.PhaseTearingDown {
			continue
		}

		mountpoint := entry.TypedSpec().Target

		ctrl.cancelScrub(mountpoint)

		// While the scrub task is still around we keep the finalizer to block
		// the unmount during an in-flight scrub. The finalizer is removed only
		// once the task has been torn down. This must happen here (and not only
		// on a completed TaskStatus in processStatuses) because the task may be
		// destroyed and dropped from ctrl.tasks before its completed status is
		// observed, which would otherwise leak the finalizer.
		if _, ok := ctrl.tasks[mountpoint]; ok {
			continue
		}

		if entry.Metadata().Finalizers().Has(ctrl.Name()) {
			if err := r.RemoveFinalizer(ctx, entry.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error removing finalizer: %w", err)
			}
		}
	}

	return nil
}

func (ctrl *FSScrubController) cancelScrub(mountpoint string) {
	if s, ok := ctrl.schedule[mountpoint]; ok && s.stop != nil {
		s.stop()
	}

	if _, ok := ctrl.tasks[mountpoint]; ok {
		ctrl.tasks[mountpoint] = scrubTask{
			Args:       ctrl.tasks[mountpoint].Args,
			Destroying: true,
		}
	}

	delete(ctrl.status, mountpoint)
	delete(ctrl.schedule, mountpoint)
}

func (ctrl *FSScrubController) createScrubTask(ctx context.Context, logger *zap.Logger, mountpoint string, opts []string, r controller.Runtime) error {
	args := make([]string, 0, 3+len(opts)+1)
	args = append(args, "/usr/sbin/xfs_scrub", "-T", "-v")
	args = append(args, opts...)
	args = append(args, mountpoint)

	mountStatus, err := ctrl.getMountStatus(ctx, r, mountpoint)
	if err != nil {
		return err
	}

	if mountStatus == nil || mountStatus.Metadata().Phase() == resource.PhaseTearingDown {
		return fmt.Errorf("not mounted or unmounting")
	}

	if _, ok := ctrl.tasks[mountpoint]; ok {
		return fmt.Errorf("already running")
	}

	if !mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error adding finalizer: %w", err)
		}

		logger.Warn("added finalizer to mount status", zap.String("mountpoint", mountpoint), zap.String("finalizer", ctrl.Name()))
	}

	logger.Warn("creating scrub task", zap.String("task", ctrl.status[mountpoint].id), zap.String("mountpoint", mountpoint))

	ctrl.tasks[mountpoint] = scrubTask{
		Args: args,
	}

	return err
}

func (ctrl *FSScrubController) getMountStatus(ctx context.Context, r controller.Runtime, mountpoint string) (*block.MountStatus, error) {
	mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error getting mount statuses to obtain finalizers: %w", err)
	}

	for entry := range mountStatuses.All() {
		if entry.TypedSpec().Target == mountpoint {
			return entry, nil
		}
	}

	return nil, nil
}
