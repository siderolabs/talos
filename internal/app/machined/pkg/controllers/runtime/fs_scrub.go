// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

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
	return "runtime.FSScrubController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FSScrubController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.FSScrubConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *FSScrubController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtimeres.FSScrubStatusType,
			Kind: controller.OutputExclusive,
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
			if err := ctrl.runScrub(mountpoint, []string{}); err != nil {
				logger.Error("error running filesystem scrub", zap.Error(err))
			}
		case <-r.EventCh():
			err := ctrl.updateSchedule(ctx, r)
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

	presentStatuses, err := safe.ReaderListAll[*runtimeres.FSScrubStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting existing FS scrub statuses: %w", err)
	}

	for entry := range presentStatuses.All() {
		if _, ok := ctrl.status[entry.TypedSpec().Mountpoint]; !ok {
			if err := r.Destroy(ctx, runtimeres.NewFSScrubStatus(entry.Metadata().ID()).Metadata()); err != nil {
				return fmt.Errorf("error destroying old FS scrub status: %w", err)
			}
		}
	}

	for _, entry := range ctrl.status {
		if err := safe.WriterModify(ctx, r, runtimeres.NewFSScrubStatus(entry.id), func(status *runtimeres.FSScrubStatus) error {
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

	if err := safe.CleanupOutputs[*runtimeres.FSScrubStatus](ctx, r); err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo,cyclop
func (ctrl *FSScrubController) updateSchedule(ctx context.Context, r controller.Runtime) error {
	volumesStatus, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting volume status: %w", err)
	}

	volumes := volumesStatus.All()

	// Deschedule scrubs for volumes that are no longer mounted.
	for mountpoint := range ctrl.schedule {
		isMounted := false

		for item := range volumes {
			vol := item.TypedSpec()

			volumeConfig, err := safe.ReaderGetByID[*block.VolumeConfig](ctx, r, item.Metadata().ID())
			if err != nil {
				return fmt.Errorf("error getting volume config: %w", err)
			}

			if volumeConfig.TypedSpec().Mount.TargetPath == mountpoint && vol.Phase == block.VolumePhaseReady {
				isMounted = true

				break
			}
		}

		if !isMounted {
			ctrl.cancelScrub(mountpoint)
		}
	}

	cfg, err := safe.ReaderListAll[*runtimeres.FSScrubConfig](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting scrub config: %w", err)
		}
	}

	for item := range volumes {
		vol := item.TypedSpec()

		if vol.Phase != block.VolumePhaseReady {
			continue
		}

		if vol.Filesystem != block.FilesystemTypeXFS {
			continue
		}

		volumeConfig, err := safe.ReaderGetByID[*block.VolumeConfig](ctx, r, item.Metadata().ID())
		if err != nil {
			return fmt.Errorf("error getting volume config: %w", err)
		}

		mountpoint := volumeConfig.TypedSpec().Mount.TargetPath

		var period *time.Duration

		for fs := range cfg.All() {
			if fs.TypedSpec().Mountpoint == mountpoint {
				period = &fs.TypedSpec().Period
			}
		}

		_, ok := ctrl.schedule[mountpoint]

		if period == nil {
			if ok {
				ctrl.cancelScrub(mountpoint)
			}

			continue
		}

		if !ok {
			firstTimeout := time.Duration(rand.Int64N(int64(period.Seconds()))) * time.Second

			// When scheduling the first scrub, we use a random time to avoid all scrubs running in a row.
			// After the first scrub, we use the period defined in the config.
			cb := func() {
				ctrl.c <- mountpoint
				ctrl.schedule[mountpoint].timer.Reset(ctrl.schedule[mountpoint].period)
			}

			ctrl.schedule[mountpoint] = scrubSchedule{
				mountpoint: mountpoint,
				period:     *period,
				timer:      time.AfterFunc(firstTimeout, cb),
			}

			ctrl.status[mountpoint] = scrubStatus{
				id:         item.Metadata().ID(),
				mountpoint: mountpoint,
				period:     *period,
				time:       time.Now().Add(firstTimeout),
				duration:   0,
				result:     fmt.Errorf("scheduled"),
			}
		} else if ctrl.schedule[mountpoint].period != *period {
			// reschedule if period has changed
			ctrl.schedule[mountpoint].timer.Stop()
			ctrl.schedule[mountpoint].timer.Reset(*period)
			ctrl.schedule[mountpoint] = scrubSchedule{
				period: *period,
				timer:  ctrl.schedule[mountpoint].timer,
			}

			ctrl.status[mountpoint] = scrubStatus{
				id:         item.Metadata().ID(),
				mountpoint: mountpoint,
				period:     *period,
				time:       ctrl.status[mountpoint].time,
				duration:   ctrl.status[mountpoint].duration,
				result:     ctrl.status[mountpoint].result,
			}
		}
	}

	return err
}

func (ctrl *FSScrubController) cancelScrub(mountpoint string) {
	ctrl.schedule[mountpoint].timer.Stop()
	delete(ctrl.schedule, mountpoint)
	delete(ctrl.status, mountpoint)
}

func (ctrl *FSScrubController) runScrub(mountpoint string, opts []string) error {
	args := []string{"/usr/sbin/xfs_scrub", "-T", "-v"}
	args = append(args, opts...)
	args = append(args, mountpoint)

	r := process.NewRunner(
		false,
		&runner.Args{
			ID:          "fs_scrub",
			ProcessArgs: args,
		},
		runner.WithLoggingManager(ctrl.Runtime.Logging()),
		runner.WithEnv(environment.Get(ctrl.Runtime.Config())),
		runner.WithOOMScoreAdj(-999),
		runner.WithDroppedCapabilities(constants.XFSScrubDroppedCapabilities),
		runner.WithPriority(19),
		runner.WithIOPriority(runner.IoprioClassIdle, 7),
		runner.WithSchedulingPolicy(runner.SchedulingPolicyIdle),
	)

	start := time.Now()

	err := r.Run(func(s events.ServiceState, msg string, args ...any) {})

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
