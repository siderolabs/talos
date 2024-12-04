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

	"github.com/siderolabs/gen/optional"
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
	period time.Duration
	timer  *time.Timer
}

// FSScrubController watches v1alpha1.Config and schedules filesystem online check tasks.
type FSScrubController struct {
	Runtime  runtime.Runtime
	schedule map[string]scrubSchedule
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
			ID:        optional.Some(runtimeres.FSScrubConfigID),
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
	return []controller.Output{}
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
	ctrl.c = make(chan string, 5)

	for {
		select {
		case <-ctx.Done():
			return nil
		case mountpoint := <-ctrl.c:
			if err := ctrl.runScrub(mountpoint, []string{}); err != nil {
				logger.Error("!!! scrub !!! error running filesystem scrub", zap.Error(err))
			}

			continue
		case <-r.EventCh():
			err := ctrl.updateSchedule(ctx, r, logger)
			if err != nil {
				return err
			}
		}
	}
}

func (ctrl *FSScrubController) updateSchedule(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	cfg, err := safe.ReaderGetByID[*runtimeres.FSScrubConfig](ctx, r, runtimeres.FSScrubConfigID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting scrub config: %w", err)
		}
	}

	if cfg == nil {
		logger.Warn("!!! scrub !!! no config")

		for mountpoint, task := range ctrl.schedule {
			task.timer.Stop()
			delete(ctrl.schedule, mountpoint)
		}

		return nil
	}

	volumesStatus, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting volume status: %w", err)
	}

	logger.Warn("!!! scrub !!! reading volume status")
	for item := range volumesStatus.All() {
		vol := item.TypedSpec()

		logger.Warn("!!! scrub !!! volume status", zap.Reflect("volume", vol))

		if vol.Phase != block.VolumePhaseReady {
			logger.Warn("!!! scrub !!! vol.Phase != block.VolumePhaseReady", zap.Reflect("item", vol))

			continue
		}

		if vol.Filesystem != block.FilesystemTypeXFS {
			logger.Warn("!!! scrub !!! vol.Filesystem != block.FilesystemTypeXFS", zap.Reflect("item", vol))

			continue
		}

		volumeConfig, err := safe.ReaderGetByID[*block.VolumeConfig](ctx, r, item.Metadata().ID())
		if err != nil {
			return fmt.Errorf("!!! scrub !!! error getting volume config: %w", err)
		}

		mountpoint := volumeConfig.TypedSpec().Mount.TargetPath

		var period *time.Duration

		for _, fs := range cfg.TypedSpec().Filesystems {
			if fs.Mountpoint == mountpoint {
				period = &fs.Period
			}
		}

		if period == nil {
			logger.Warn("!!! scrub !!! not in config", zap.String("mountpoint", mountpoint))

			return nil
		}

		_, ok := ctrl.schedule[mountpoint]

		if !ok {
			firstTimeout := time.Duration(rand.Int64N(int64(period.Seconds()))) * time.Second
			logger.Warn("!!! scrub !!! firstTimeout", zap.Duration("firstTimeout", firstTimeout))

			// When scheduling the first scrub, we use a random time to avoid all scrubs running in a row.
			// After the first scrub, we use the period defined in the config.
			cb := func() {
				logger.Warn("!!! scrub !!! ding", zap.String("path", mountpoint))
				ctrl.c <- mountpoint
				ctrl.schedule[mountpoint].timer.Reset(ctrl.schedule[mountpoint].period)
			}

			ctrl.schedule[mountpoint] = scrubSchedule{
				period: *period,
				timer:  time.AfterFunc(firstTimeout, cb),
			}

			logger.Warn("!!! scrub !!! scheduled", zap.String("path", mountpoint), zap.Duration("period", *period))
		} else {
			// reschedule if period has changed
			logger.Warn("!!! scrub !!! reschedule", zap.String("path", mountpoint), zap.Duration("period", *period))
			if ctrl.schedule[mountpoint].period != *period {
				ctrl.schedule[mountpoint].timer.Stop()
				ctrl.schedule[mountpoint].timer.Reset(*period)
				ctrl.schedule[mountpoint] = scrubSchedule{
					period: *period,
					timer:  ctrl.schedule[mountpoint].timer,
				}
			}
		}
	}

	return err
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

	return r.Run(func(s events.ServiceState, msg string, args ...any) {})
}
