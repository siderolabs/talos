// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

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
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// FSScrubController performs scheduled filesystem scrubs on mounted volumes.
//
// It watches FSScrubSchedule resources, wakes up when the next scheduled scrub is due, and,
// if the volume is mounted, runs the scrub on the mounted filesystem.
//
// Scrubs are run one at a time, directly by this controller.
type FSScrubController struct {
	// Runtime provides access to the logging manager and machine config for the scrub process.
	Runtime runtime.Runtime

	// ScrubFunc runs the scrub on the mounted filesystem at the given target path,
	// defaults to running xfs_scrub.
	//
	// It is overridable for testing.
	ScrubFunc func(ctx context.Context, logger *zap.Logger, target string) error

	// lastScrubbed tracks the most recent scrub slot handled per volume ID.
	//
	// A volume present in the map but with a zero/earlier value than the current slot is due.
	// A volume absent from the map has not been observed yet, and its current slot is skipped to
	// avoid scrubbing right after (re)start.
	lastScrubbed map[string]time.Time

	// status tracks the outcome of the most recent scrub per volume ID.
	status map[string]scrubStatus
}

type scrubStatus struct {
	mountpoint string
	interval   time.Duration
	time       time.Time
	duration   time.Duration
	result     error
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
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *FSScrubController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.lastScrubbed == nil {
		ctrl.lastScrubbed = map[string]time.Time{}
	}

	if ctrl.status == nil {
		ctrl.status = map[string]scrubStatus{}
	}

	if ctrl.ScrubFunc == nil {
		ctrl.ScrubFunc = ctrl.runXFSScrub
	}

	// The timer that tracks the next scrub time
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-timer.C:
		}

		// re-reconcile as long as events were consumed while a scrub was running, as the state
		// listed at the beginning of the reconcile might be outdated by the time the scrub is done.
		for {
			resync, err := ctrl.reconcile(ctx, r, logger, timer)
			if err != nil {
				return err
			}

			if !resync {
				break
			}
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo,cyclop
func (ctrl *FSScrubController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger, timer *time.Timer) (resync bool, err error) {
	schedules, err := safe.ReaderListAll[*block.FSScrubSchedule](ctx, r)
	if err != nil {
		return false, fmt.Errorf("failed to list scrub schedules: %w", err)
	}

	// build a map of mounted volumes by volume ID.
	mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
	if err != nil {
		return false, fmt.Errorf("failed to list mount statuses: %w", err)
	}

	mountStatusByVolume := map[string]*block.MountStatus{}

	for mountStatus := range mountStatuses.All() {
		if mountStatus.Metadata().Phase() != resource.PhaseRunning {
			// release a possibly stale finalizer (e.g. left over after a crash mid-scrub) so the
			// volume can be unmounted; a finalizer is only legitimately held while a scrub is
			// running, and scrubs are performed synchronously within a single reconcile.
			if mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
				if err = r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
					return false, fmt.Errorf("failed to remove finalizer from mount status %q: %w", mountStatus.Metadata().ID(), err)
				}
			}

			continue
		}

		mountStatusByVolume[mountStatus.TypedSpec().Spec.VolumeID] = mountStatus
	}

	now := time.Now()

	var earliestNext time.Time

	scheduledVolumes := map[string]struct{}{}

	for schedule := range schedules.All() {
		volumeID := schedule.Metadata().ID()
		interval := schedule.TypedSpec().Interval

		if interval <= 0 {
			continue
		}

		scheduledVolumes[volumeID] = struct{}{}

		// the schedule's NextScrub is a slot on the lattice, use it as the anchor so the
		// runner does not need to recompute the (node-salted) schedule hash itself.
		anchor := schedule.TypedSpec().NextScrub

		currentSlot := block.ScheduleSlotBefore(anchor, interval, now)
		nextSlot := block.ScheduleSlotAfter(anchor, interval, now)

		if earliestNext.IsZero() || nextSlot.Before(earliestNext) {
			earliestNext = nextSlot
		}

		lastScrubbed, observed := ctrl.lastScrubbed[volumeID]
		if !observed {
			// first time we observe this schedule: skip the current slot to avoid scrubbing
			// right after (re)start, the next slot will be handled normally.
			ctrl.lastScrubbed[volumeID] = currentSlot

			continue
		}

		if !lastScrubbed.Before(currentSlot) {
			// already handled the current slot
			continue
		}

		// the current slot is due.
		ctrl.lastScrubbed[volumeID] = currentSlot

		mountStatus := mountStatusByVolume[volumeID]
		if mountStatus == nil {
			logger.Debug("skipping scrub, volume is not mounted", zap.String("volume", volumeID))

			continue
		}

		if mountStatus.TypedSpec().ReadOnly {
			logger.Debug("skipping scrub, volume is mounted read-only", zap.String("volume", volumeID))

			continue
		}

		if mountStatus.TypedSpec().Detached {
			logger.Debug("skipping scrub, volume is mounted detached", zap.String("volume", volumeID))

			continue
		}

		didResync, err := ctrl.scrub(ctx, r, logger, mountStatus, interval)
		if err != nil {
			return false, fmt.Errorf("failed to scrub volume %q: %w", volumeID, err)
		}

		resync = resync || didResync
	}

	// drop tracking and status for volumes which no longer have a schedule.
	for volumeID := range ctrl.lastScrubbed {
		if _, ok := scheduledVolumes[volumeID]; !ok {
			delete(ctrl.lastScrubbed, volumeID)
			delete(ctrl.status, volumeID)
		}
	}

	if err = ctrl.reportStatus(ctx, r); err != nil {
		return false, err
	}

	// re-arm the timer to wake up once the earliest next slot passes.
	select {
	case <-timer.C:
	default:
	}

	if !earliestNext.IsZero() {
		timer.Reset(max(time.Until(earliestNext), time.Second))
	}

	return resync, nil
}

// scrub runs the scrub on the mounted volume, holding a finalizer on the mount status
// for the duration of the operation so the volume is not unmounted while being scrubbed.
//
// The scrub is aborted if the mount status starts tearing down while the scrub is running,
// so an in-flight scrub doesn't block the unmount.
//
// The result is recorded in ctrl.status; a failed scrub is not a controller error.
//
//nolint:gocyclo
func (ctrl *FSScrubController) scrub(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, mountStatus *block.MountStatus, interval time.Duration,
) (resync bool, err error) {
	volumeID := mountStatus.TypedSpec().Spec.VolumeID
	target := mountStatus.TypedSpec().Target

	if target == "" {
		logger.Debug("skipping scrub, mount target is not available", zap.String("volume", volumeID))

		return false, nil
	}

	if err = r.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
		return false, fmt.Errorf("failed to add finalizer to mount status %q: %w", mountStatus.Metadata().ID(), err)
	}

	defer func() {
		if finalizerErr := r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); finalizerErr != nil {
			logger.Error("failed to remove finalizer from mount status", zap.String("mount_status", mountStatus.Metadata().ID()), zap.Error(finalizerErr))
		}
	}()

	logger.Info("scrubbing volume", zap.String("volume", volumeID), zap.String("target", target))

	scrubCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()

	errCh := make(chan error, 1)

	go func() {
		errCh <- ctrl.ScrubFunc(scrubCtx, logger, target)
	}()

	var scrubErr error

waitLoop:
	for {
		select {
		case scrubErr = <-errCh:
			break waitLoop
		case <-r.EventCh():
			resync = true

			// abort the scrub if the mount status started tearing down, so the in-flight scrub
			// doesn't block the unmount.
			ms, msErr := safe.ReaderGetByID[*block.MountStatus](ctx, r, mountStatus.Metadata().ID())
			if msErr != nil && !state.IsNotFoundError(msErr) {
				cancel()
				<-errCh

				return resync, fmt.Errorf("failed to get mount status %q: %w", mountStatus.Metadata().ID(), msErr)
			}

			if msErr != nil || ms.Metadata().Phase() == resource.PhaseTearingDown {
				logger.Info("aborting scrub, volume is being unmounted", zap.String("volume", volumeID))

				cancel()

				scrubErr = <-errCh

				break waitLoop
			}
		}
	}

	ctrl.status[volumeID] = scrubStatus{
		mountpoint: target,
		interval:   interval,
		time:       start,
		duration:   time.Since(start),
		result:     scrubErr,
	}

	if scrubErr != nil {
		logger.Error(
			"filesystem scrub failed",
			zap.String("volume", volumeID),
			zap.String("target", target),
			zap.Error(scrubErr),
		)
	} else {
		logger.Info(
			"filesystem scrub completed",
			zap.String("volume", volumeID),
			zap.String("target", target),
			zap.Duration("duration", time.Since(start)),
		)
	}

	return resync, nil
}

// reportStatus writes FSScrubStatus resources for the tracked scrub outcomes.
func (ctrl *FSScrubController) reportStatus(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	for volumeID, entry := range ctrl.status {
		if err := safe.WriterModify(ctx, r, block.NewFSScrubStatus(volumeID), func(status *block.FSScrubStatus) error {
			status.TypedSpec().Mountpoint = entry.mountpoint
			status.TypedSpec().Interval = entry.interval
			status.TypedSpec().Time = entry.time
			status.TypedSpec().Duration = entry.duration

			if entry.result != nil {
				status.TypedSpec().Status = entry.result.Error()
			} else {
				status.TypedSpec().Status = "success"
			}

			return nil
		}); err != nil {
			return fmt.Errorf("failed to update filesystem scrub status: %w", err)
		}
	}

	if err := safe.CleanupOutputs[*block.FSScrubStatus](ctx, r); err != nil {
		return fmt.Errorf("failed to clean up filesystem scrub statuses: %w", err)
	}

	return nil
}

// runXFSScrub runs xfs_scrub on the mounted filesystem at the given target path.
//
// The process runs with minimal privileges (only the capabilities xfs_scrub needs) and
// the lowest CPU/IO priority; it is stopped when the context is canceled.
func (ctrl *FSScrubController) runXFSScrub(ctx context.Context, logger *zap.Logger, target string) error {
	taskRunner := process.NewRunner(
		false, &runner.Args{
			ID:          "fs_scrub",
			ProcessArgs: []string{"/usr/sbin/xfs_scrub", "-T", "-v", target},
		},
		runner.WithLoggingManager(ctrl.Runtime.Logging()),
		runner.WithEnv(environment.Get(ctrl.Runtime.Config())),
		runner.WithCapabilities(constants.XFSScrubCapabilities),
		runner.WithPriority(constants.FilesystemScrubPriority),
		runner.WithIOPriority(runner.IoprioClassIdle, 7),
		runner.WithSchedulingPolicy(runner.SchedulingPolicyIdle),
		runner.WithSelinuxLabel(constants.SelinuxLabelMachined),
	)

	if err := taskRunner.Open(); err != nil {
		return fmt.Errorf("failed to open scrub runner: %w", err)
	}

	defer taskRunner.Close() //nolint:errcheck

	errCh := make(chan error, 1)

	go func() {
		errCh <- taskRunner.Run(
			func(events.ServiceState, string, ...any) {},
			func(string, int32, bool) error { return nil },
		)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		if err := taskRunner.Stop(); err != nil {
			logger.Error("failed to stop the scrub process", zap.Error(err))
		}

		<-errCh

		return ctx.Err()
	}
}
