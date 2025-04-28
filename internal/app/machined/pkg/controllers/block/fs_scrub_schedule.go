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

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type ScrubSchedule struct {
	id         string
	mountpoint string
	period     time.Duration
	startTime  time.Time // first time to start, selected randomly
}

// FSScrubScheduleController watches v1alpha1.Config and schedules filesystem online check tasks.
type FSScrubScheduleController struct {
	schedule map[string]ScrubSchedule
}

// Name implements controller.Controller interface.
func (ctrl *FSScrubScheduleController) Name() string {
	return "block.FSScrubScheduleController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FSScrubScheduleController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.FSScrubConfigType,
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
func (ctrl *FSScrubScheduleController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.FSScrubScheduleType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *FSScrubScheduleController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.schedule = make(map[string]ScrubSchedule)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			err := ctrl.updateSchedule(ctx, r)
			if err != nil {
				return err
			}
		}

		if err := ctrl.updateOutputs(ctx, r); err != nil {
			return err
		}
	}
}

func (ctrl *FSScrubScheduleController) updateOutputs(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	presentEntries, err := safe.ReaderListAll[*block.FSScrubSchedule](ctx, r)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting existing FS scrub schedules: %w", err)
	}

	for entry := range presentEntries.All() {
		if _, ok := ctrl.schedule[entry.TypedSpec().Mountpoint]; !ok {
			if err := r.Destroy(ctx, block.NewFSScrubSchedule(entry.Metadata().ID()).Metadata()); err != nil {
				return fmt.Errorf("error destroying old FS scrub schedules: %w", err)
			}
		}
	}

	for _, entry := range ctrl.schedule {
		if err := safe.WriterModify(ctx, r, block.NewFSScrubSchedule(entry.id), func(status *block.FSScrubSchedule) error {
			status.TypedSpec().Mountpoint = entry.mountpoint
			status.TypedSpec().Period = entry.period
			status.TypedSpec().StartTime = entry.startTime

			return nil
		}); err != nil {
			return fmt.Errorf("error updating filesystem scrub schedules: %w", err)
		}
	}

	if err := safe.CleanupOutputs[*block.FSScrubSchedule](ctx, r); err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo,cyclop
func (ctrl *FSScrubScheduleController) updateSchedule(ctx context.Context, r controller.Runtime) error {
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
			delete(ctrl.schedule, mountpoint)
		}
	}

	cfg, err := safe.ReaderListAll[*block.FSScrubConfig](ctx, r)
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

		if period == nil {
			delete(ctrl.schedule, mountpoint)
			continue
		}

		if _, ok := ctrl.schedule[mountpoint]; ok && ctrl.schedule[mountpoint].period == *period {
			continue
		}

		firstTimeout := time.Duration(rand.Int64N(int64(period.Seconds()))) * time.Second

		ctrl.schedule[mountpoint] = ScrubSchedule{
			id:         item.Metadata().ID(),
			mountpoint: mountpoint,
			period:     *period,
			startTime:  time.Now().Add(firstTimeout),
		}
	}

	return err
}
