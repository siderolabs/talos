// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/dustin/go-humanize"
	"github.com/siderolabs/go-blockdevice/v2/fstrim"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeTrimController performs scheduled trims (fstrim) on mounted volumes.
//
// It watches VolumeTrimSchedule resources, wakes up when the next scheduled trim is due, and,
// if the volume is mounted, performs the trim on the mounted filesystem.
type VolumeTrimController struct {
	// TrimFunc performs the trim on the mounted filesystem at the given target path,
	// defaults to fstrim.Fstrim.
	//
	// It is overridable for testing.
	TrimFunc func(target string) (uint64, error)

	// lastTrimmed tracks the most recent trim slot handled per volume ID.
	//
	// A volume present in the map but with a zero/earlier value than the current slot is due.
	// A volume absent from the map has not been observed yet, and its current slot is skipped to
	// avoid trimming right after (re)start.
	lastTrimmed map[string]time.Time
}

// Name implements controller.Controller interface.
func (ctrl *VolumeTrimController) Name() string {
	return "block.VolumeTrimController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VolumeTrimController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeTrimScheduleType,
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
func (ctrl *VolumeTrimController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *VolumeTrimController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.lastTrimmed == nil {
		ctrl.lastTrimmed = map[string]time.Time{}
	}

	if ctrl.TrimFunc == nil {
		ctrl.TrimFunc = fstrim.Fstrim
	}

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	timer.Stop()

	drainTimer := func() {
		select {
		case <-timer.C:
		default:
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-timer.C:
		}

		schedules, err := safe.ReaderListAll[*block.VolumeTrimSchedule](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list trim schedules: %w", err)
		}

		// build a map of mounted volumes by volume ID.
		mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list mount statuses: %w", err)
		}

		mountStatusByVolume := map[string]*block.MountStatus{}

		for mountStatus := range mountStatuses.All() {
			if mountStatus.Metadata().Phase() != resource.PhaseRunning {
				// release a possibly stale finalizer (e.g. left over after a crash mid-trim) so the
				// volume can be unmounted; trims are performed synchronously within a single reconcile,
				// so we never legitimately hold a finalizer across reconciles.
				if mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
					if err = r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
						return fmt.Errorf("failed to remove finalizer from mount status %q: %w", mountStatus.Metadata().ID(), err)
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

			// the schedule's NextTrim is a slot on the lattice, use it as the anchor so the
			// runner does not need to recompute the (node-salted) schedule hash itself.
			anchor := schedule.TypedSpec().NextTrim

			currentSlot := block.ScheduleSlotBefore(anchor, interval, now)
			nextSlot := block.ScheduleSlotAfter(anchor, interval, now)

			if earliestNext.IsZero() || nextSlot.Before(earliestNext) {
				earliestNext = nextSlot
			}

			lastTrimmed, observed := ctrl.lastTrimmed[volumeID]
			if !observed {
				// first time we observe this schedule: skip the current slot to avoid trimming
				// right after (re)start, the next slot will be handled normally.
				ctrl.lastTrimmed[volumeID] = currentSlot

				continue
			}

			if !lastTrimmed.Before(currentSlot) {
				// already handled the current slot
				continue
			}

			// the current slot is due.
			ctrl.lastTrimmed[volumeID] = currentSlot

			mountStatus := mountStatusByVolume[volumeID]
			if mountStatus == nil {
				logger.Debug("skipping trim, volume is not mounted", zap.String("volume", volumeID))

				continue
			}

			if mountStatus.TypedSpec().ReadOnly {
				logger.Debug("skipping trim, volume is mounted read-only", zap.String("volume", volumeID))

				continue
			}

			if mountStatus.TypedSpec().Detached {
				logger.Debug("skipping trim, volume is mounted detached", zap.String("volume", volumeID))

				continue
			}

			if err = ctrl.trim(ctx, r, logger, mountStatus); err != nil {
				return fmt.Errorf("failed to trim volume %q: %w", volumeID, err)
			}
		}

		// drop tracking for volumes which no longer have a schedule.
		for volumeID := range ctrl.lastTrimmed {
			if _, ok := scheduledVolumes[volumeID]; !ok {
				delete(ctrl.lastTrimmed, volumeID)
			}
		}

		drainTimer()

		if !earliestNext.IsZero() {
			timer.Reset(max(time.Until(earliestNext), time.Second))
		}

		r.ResetRestartBackoff()
	}
}

// trim performs the fstrim on the mounted volume, holding a finalizer on the mount status
// for the duration of the operation so the volume is not unmounted while being trimmed.
func (ctrl *VolumeTrimController) trim(ctx context.Context, r controller.Runtime, logger *zap.Logger, mountStatus *block.MountStatus) error {
	volumeID := mountStatus.TypedSpec().Spec.VolumeID
	target := mountStatus.TypedSpec().Target

	if target == "" {
		logger.Debug("skipping trim, mount target is not available", zap.String("volume", volumeID))

		return nil
	}

	if err := r.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("failed to add finalizer to mount status %q: %w", mountStatus.Metadata().ID(), err)
	}

	defer func() {
		if err := r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
			logger.Error("failed to remove finalizer from mount status", zap.String("mount_status", mountStatus.Metadata().ID()), zap.Error(err))
		}
	}()

	trimmed, err := ctrl.TrimFunc(target)
	if err != nil {
		if errors.Is(err, fstrim.ErrNotSupported) {
			logger.Warn("trim not supported for volume", zap.String("volume", volumeID), zap.String("target", target))

			return nil
		}

		return fmt.Errorf("failed to trim mount %q: %w", mountStatus.Metadata().ID(), err)
	}

	logger.Info("trimmed volume",
		zap.String("volume", volumeID),
		zap.String("target", target),
		zap.String("trimmed", humanize.Bytes(trimmed)),
	)

	return nil
}
