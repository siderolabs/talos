// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/gen/xslices"
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

// Run implements controller.Controller interface.
//
//nolint:gocyclo
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
				func(v *block.VolumeStatus) *block.VolumeStatus {
					return v
				},
			),
			func(v *block.VolumeStatus) (string, *block.VolumeStatusSpec) {
				return v.Metadata().ID(), v.TypedSpec()
			},
		)

		volumeMountRequests, err := safe.ReaderListAll[*block.VolumeMountRequest](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read volume mount requests: %w", err)
		}

		desiredMountRequests := map[string]*block.MountRequestSpec{}

		for volumeMountRequest := range volumeMountRequests.All() {
			volumeStatus, ok := volumeStatusMap[volumeMountRequest.TypedSpec().VolumeID]
			if !ok || volumeStatus.Phase != block.VolumePhaseReady {
				continue
			}

			if _, exists := desiredMountRequests[volumeMountRequest.Metadata().ID()]; !exists {
				desiredMountRequests[volumeMountRequest.Metadata().ID()] = &block.MountRequestSpec{
					Source: volumeStatus.MountLocation,
				}
			}
		}

	}
}
