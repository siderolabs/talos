// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// MountRequestController provides mount requests based on VolumeMountRequests and VolumeStatuses.
type MountRequestController struct{}

// Name implements controller.Controller interface.
func (ctrl *MountRequestController) Name() string {
	return "block.MountRequestController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MountRequestController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.MountRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MountRequestController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.MountRequestType,
			Kind: controller.OutputExclusive,
		},
	}
}

func identity[T any](v T) T {
	return v
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *MountRequestController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read volume statuses: %w", err)
		}

		volumeStatusMap := xslices.ToMap(
			safe.ToSlice(
				volumeStatuses,
				identity,
			),
			func(v *block.VolumeStatus) (string, *block.VolumeStatus) {
				return v.Metadata().ID(), v
			},
		)

		volumeMountRequests, err := safe.ReaderListAll[*block.VolumeMountRequest](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read volume mount requests: %w", err)
		}

		desiredMountRequests := map[string]*block.MountRequestSpec{}

		for volumeMountRequest := range volumeMountRequests.All() {
			volumeID := volumeMountRequest.TypedSpec().VolumeID

			volumeStatus, ok := volumeStatusMap[volumeID]
			if !ok || volumeStatus.TypedSpec().Phase != block.VolumePhaseReady || volumeStatus.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if _, exists := desiredMountRequests[volumeID]; !exists {
				desiredMountRequests[volumeID] = &block.MountRequestSpec{
					VolumeID: volumeID,
					ReadOnly: volumeMountRequest.TypedSpec().ReadOnly,
				}
			}

			desiredMountRequest := desiredMountRequests[volumeID]
			desiredMountRequest.Requesters = append(desiredMountRequest.Requesters, volumeMountRequest.TypedSpec().Requester)
			desiredMountRequest.RequesterIDs = append(desiredMountRequest.RequesterIDs, volumeMountRequest.Metadata().ID())
			desiredMountRequest.ReadOnly = desiredMountRequest.ReadOnly && volumeMountRequest.TypedSpec().ReadOnly // read-only if all requesters are read-only
			desiredMountRequest.ParentMountID = volumeStatus.TypedSpec().MountSpec.ParentID
		}

		// list and figure out what to do with existing mount requests
		mountRequests, err := safe.ReaderListAll[*block.MountRequest](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read mount requests: %w", err)
		}

		// perform cleanup of mount requests which should be cleaned up
		for mountRequest := range mountRequests.All() {
			tearingDown := mountRequest.Metadata().Phase() == resource.PhaseTearingDown
			shouldBeDestroyed := desiredMountRequests[mountRequest.Metadata().ID()] == nil

			if tearingDown || shouldBeDestroyed {
				okToDestroy, err := r.Teardown(ctx, mountRequest.Metadata())
				if err != nil {
					return fmt.Errorf("failed to teardown mount request %q: %w", mountRequest.Metadata().ID(), err)
				}

				if okToDestroy {
					if err = r.Destroy(ctx, mountRequest.Metadata()); err != nil {
						return fmt.Errorf("failed to destroy mount request %q: %w", mountRequest.Metadata().ID(), err)
					}
				} else if !shouldBeDestroyed {
					// previous mount request version is still being torn down
					delete(desiredMountRequests, mountRequest.Metadata().ID())
				}
			}
		}

		// create/update mount requests
		for id, desiredMountRequest := range desiredMountRequests {
			if err = safe.WriterModify(
				ctx, r, block.NewMountRequest(block.NamespaceName, id),
				func(mr *block.MountRequest) error {
					*mr.TypedSpec() = *desiredMountRequest

					return nil
				},
			); err != nil {
				return fmt.Errorf("failed to create/update mount request %q: %w", id, err)
			}
		}

		r.ResetRestartBackoff()
	}
}
