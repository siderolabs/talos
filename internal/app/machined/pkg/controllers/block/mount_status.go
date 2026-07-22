// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// MountStatusController provides mount requests based on VolumeMountRequests and VolumeStatuses.
type MountStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *MountStatusController) Name() string {
	return "block.MountStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MountStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.MountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MountStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeMountStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *MountStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read volume mount requests: %w", err)
		}

		for mountStatus := range mountStatuses.All() {
			switch mountStatus.Metadata().Phase() {
			case resource.PhaseRunning:
				// always put our own finalizer
				if !mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
					if err = r.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
						return fmt.Errorf("failed to add finalizer to mount status %q: %w", mountStatus.Metadata().ID(), err)
					}
				}

				// first, clean up volume mount statuses which do not match any existing requesters, and finish
				// tearing down the ones which are already tearing down (a requester might have come back
				// while its previous volume mount status was still being destroyed)
				volumeMountStatuses, err := safe.ReaderListAll[*block.VolumeMountStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual("mount-status-id", mountStatus.Metadata().ID())))
				if err != nil {
					return fmt.Errorf("failed to read volume mount statuses for mount status %q: %w", mountStatus.Metadata().ID(), err)
				}

				for volumeMountStatus := range volumeMountStatuses.All() {
					if volumeMountStatus.Metadata().Phase() == resource.PhaseRunning &&
						slices.Contains(mountStatus.TypedSpec().Spec.RequesterIDs, volumeMountStatus.Metadata().ID()) {
						// still active
						continue
					}

					okToDestroy, err := r.Teardown(ctx, volumeMountStatus.Metadata())
					if err != nil {
						return fmt.Errorf("failed to teardown volume mount status %q: %w", volumeMountStatus.Metadata().ID(), err)
					}

					if okToDestroy {
						if err = r.Destroy(ctx, volumeMountStatus.Metadata()); err != nil {
							return fmt.Errorf("failed to destroy volume mount status %q: %w", volumeMountStatus.Metadata().ID(), err)
						}
					}
				}

				// now "explode" the mount status into volume mount statuses per requester
				for i, requester := range mountStatus.TypedSpec().Spec.Requesters {
					requestID := mountStatus.TypedSpec().Spec.RequesterIDs[i]

					if err = safe.WriterModify(
						ctx, r, block.NewVolumeMountStatus(block.NamespaceName, requestID),
						func(vms *block.VolumeMountStatus) error {
							vms.Metadata().Labels().Set("mount-status-id", mountStatus.Metadata().ID())
							vms.TypedSpec().Requester = requester
							vms.TypedSpec().Target = mountStatus.TypedSpec().Target
							vms.TypedSpec().VolumeID = mountStatus.TypedSpec().Spec.VolumeID
							vms.TypedSpec().ReadOnly = mountStatus.TypedSpec().Spec.ReadOnly
							vms.TypedSpec().Detached = mountStatus.TypedSpec().Detached
							vms.TypedSpec().DisableAccessTime = mountStatus.TypedSpec().Spec.DisableAccessTime
							vms.TypedSpec().Secure = mountStatus.TypedSpec().Spec.Secure

							// This needs to be set through accessor, and is not guaranteed to resolve to a valid root.
							vms.TypedSpec().SetRoot(mountStatus.TypedSpec().Root())

							return nil
						},
					); err != nil {
						if state.IsPhaseConflictError(err) {
							// the previous volume mount status for this requester is still tearing down, as someone
							// holds a finalizer on it; skip it for now, and retry once it is destroyed
							//
							// this should never block other requesters or other mount statuses
							logger.Debug("waiting for volume mount status to be torn down", zap.String("volume_mount_status", requestID))

							continue
						}

						return fmt.Errorf("failed to create volume mount status %q: %w", requestID, err)
					}
				}
			case resource.PhaseTearingDown:
				// we need to ensure that all volume mount statuses are torn down and destroyed
				volumeMountStatus, err := safe.ReaderListAll[*block.VolumeMountStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual("mount-status-id", mountStatus.Metadata().ID())))
				if err != nil {
					return fmt.Errorf("failed to read volume mount statuses for mount status %q: %w", mountStatus.Metadata().ID(), err)
				}

				allDestroyed := true

				for volumeMountStatus := range volumeMountStatus.All() {
					okToDestroy, err := r.Teardown(ctx, volumeMountStatus.Metadata())
					if err != nil {
						return fmt.Errorf("failed to teardown volume mount status %q: %w", volumeMountStatus.Metadata().ID(), err)
					}

					if okToDestroy {
						if err = r.Destroy(ctx, volumeMountStatus.Metadata()); err != nil {
							return fmt.Errorf("failed to destroy volume mount status %q: %w", volumeMountStatus.Metadata().ID(), err)
						}
					} else {
						allDestroyed = false
					}
				}

				if allDestroyed {
					// remove our finalizer now
					if mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
						if err = r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
							return fmt.Errorf("failed to remove finalizer from mount status %q: %w", mountStatus.Metadata().ID(), err)
						}
					}
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
