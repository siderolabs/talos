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

// VolumeTrimScheduleController builds a stable trim schedule for volumes which support trimming.
//
// It looks at all ready disk/partition volumes with a trim-capable filesystem which have trimming
// enabled (and, for encrypted volumes, only those which allow discards), and produces a
// VolumeTrimSchedule resource per volume with a stable, hash-derived next trim time.
//
// The resolved trim settings (enabled/interval/allow-discards) are carried on the VolumeStatus
// resource itself (populated from the machine config by the VolumeConfig/VolumeManager controllers).
//
// The schedule hash is salted with the node ID, so different nodes in a cluster trim at different
// times even if they have identically named volumes.
type VolumeTrimScheduleController struct{}

// Name implements controller.Controller interface.
func (ctrl *VolumeTrimScheduleController) Name() string {
	return "block.VolumeTrimScheduleController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VolumeTrimScheduleController) Inputs() []controller.Input {
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
func (ctrl *VolumeTrimScheduleController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeTrimScheduleType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *VolumeTrimScheduleController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// timer used to refresh the scheduled NextTrim values once they pass.
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
		if identity != nil {
			nodeID := identity.TypedSpec().NodeID

			var earliestNext time.Time

			//nolint:dupl
			for volumeStatus := range volumeStatuses.All() {
				if !trimEligible(volumeStatus.TypedSpec()) {
					continue
				}

				volumeID := volumeStatus.Metadata().ID()
				interval := volumeStatus.TypedSpec().TrimInterval
				filesystem := volumeStatus.TypedSpec().Filesystem

				// salt the schedule with the node ID so different nodes trim at different times.
				nextTrim := block.NextScheduledTime(nodeID+"/"+volumeID, interval, now)

				if earliestNext.IsZero() || nextTrim.Before(earliestNext) {
					earliestNext = nextTrim
				}

				if err = safe.WriterModify(ctx, r, block.NewVolumeTrimSchedule(block.NamespaceName, volumeID),
					func(schedule *block.VolumeTrimSchedule) error {
						schedule.TypedSpec().Filesystem = filesystem
						schedule.TypedSpec().Interval = interval
						schedule.TypedSpec().NextTrim = nextTrim

						return nil
					},
				); err != nil {
					return fmt.Errorf("failed to update trim schedule for volume %q: %w", volumeID, err)
				}
			}

			// re-arm the timer to refresh the schedule once the earliest NextTrim passes.
			drainTimer()

			if !earliestNext.IsZero() {
				timer.Reset(max(time.Until(earliestNext), time.Second))
			}
		}

		if err = safe.CleanupOutputs[*block.VolumeTrimSchedule](ctx, r); err != nil {
			return fmt.Errorf("failed to clean up trim schedules: %w", err)
		}
	}
}

// trimEligible reports whether a volume should be trimmed on a schedule.
func trimEligible(spec *block.VolumeStatusSpec) bool {
	if !spec.TrimEnabled || spec.TrimInterval <= 0 {
		return false
	}

	if spec.Phase != block.VolumePhaseReady {
		return false
	}

	if spec.Type != block.VolumeTypeDisk && spec.Type != block.VolumeTypePartition {
		return false
	}

	if !spec.Filesystem.SupportsTrim() {
		return false
	}

	// for encrypted volumes, only trim if discards are allowed in the encryption spec.
	if spec.EncryptionProvider != block.EncryptionProviderNone && !spec.EncryptionAllowDiscards {
		return false
	}

	return true
}
