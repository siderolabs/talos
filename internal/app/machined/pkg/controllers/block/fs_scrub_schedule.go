// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

// FSScrubScheduleController builds a stable scrub schedule for volumes which support scrubbing.
//
// It looks at all ready volumes with a scrub-capable filesystem (currently XFS) which have
// scrubbing enabled, and produces a FSScrubSchedule resource per volume with a stable,
// hash-derived next scrub time.
//
// The resolved scrub settings (enabled/interval) are carried on the VolumeStatus resource itself
// (populated from the machine config by the VolumeConfig/VolumeManager controllers).
//
// The schedule hash is salted with the node ID, so different nodes in a cluster scrub at
// different times even if they have identically named volumes.
type FSScrubScheduleController struct{}

// Name implements controller.Controller interface.
func (ctrl *FSScrubScheduleController) Name() string {
	return "block.FSScrubScheduleController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FSScrubScheduleController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        optional.Some(cluster.LocalIdentity),
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
//
//nolint:gocyclo
func (ctrl *FSScrubScheduleController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// timer used to refresh the scheduled NextScrub values once they pass.
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

		identity, err := safe.ReaderGetByID[*cluster.Identity](ctx, r, cluster.LocalIdentity)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get node identity: %w", err)
		}

		volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list volume statuses: %w", err)
		}

		now := time.Now()

		r.StartTrackingOutputs()

		// without a node identity, no schedules are produced.
		if identity == nil {
			if err = safe.CleanupOutputs[*block.FSScrubSchedule](ctx, r); err != nil {
				return fmt.Errorf("failed to clean up scrub schedules: %w", err)
			}

			continue
		}

		nodeID := identity.TypedSpec().NodeID

		var earliestNext time.Time

		//nolint:dupl
		for volumeStatus := range volumeStatuses.All() {
			if !scrubEligible(volumeStatus.TypedSpec()) {
				continue
			}

			volumeID := volumeStatus.Metadata().ID()
			interval := volumeStatus.TypedSpec().ScrubInterval
			filesystem := volumeStatus.TypedSpec().Filesystem

			// salt the schedule with the node ID so different nodes scrub at different times.
			nextScrub := block.NextScheduledTime(nodeID+"/"+volumeID, interval, now)

			if earliestNext.IsZero() || nextScrub.Before(earliestNext) {
				earliestNext = nextScrub
			}

			if err = safe.WriterModify(
				ctx, r, block.NewFSScrubSchedule(block.NamespaceName, volumeID),
				func(schedule *block.FSScrubSchedule) error {
					schedule.TypedSpec().Filesystem = filesystem
					schedule.TypedSpec().Interval = interval
					schedule.TypedSpec().NextScrub = nextScrub

					return nil
				},
			); err != nil {
				return fmt.Errorf("failed to update scrub schedule for volume %q: %w", volumeID, err)
			}
		}

		// re-arm the timer to refresh the schedule once the earliest NextScrub passes.
		drainTimer()

		if !earliestNext.IsZero() {
			timer.Reset(max(time.Until(earliestNext), time.Second))
		}

		if err = safe.CleanupOutputs[*block.FSScrubSchedule](ctx, r); err != nil {
			return fmt.Errorf("failed to clean up scrub schedules: %w", err)
		}
	}
}

// scrubEligible reports whether a volume filesystem should be scrubbed on a schedule.
func scrubEligible(spec *block.VolumeStatusSpec) bool {
	if !spec.ScrubEnabled || spec.ScrubInterval <= 0 {
		return false
	}

	if spec.Phase != block.VolumePhaseReady {
		return false
	}

	// only XFS filesystems support online scrubbing (xfs_scrub).
	return spec.Filesystem == block.FilesystemTypeXFS
}
