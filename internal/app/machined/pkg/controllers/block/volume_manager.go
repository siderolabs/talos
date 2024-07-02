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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/value"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VolumeManagerController manages volumes in the system, converting VolumeConfig resources to VolumeStatuses.
type VolumeManagerController struct{}

// Name implements controller.Controller interface.
func (ctrl *VolumeManagerController) Name() string {
	return "block.VolumeManagerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VolumeManagerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeConfigType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiskType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.SystemDiskType,
			ID:        optional.Some(block.SystemDiskID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.DevicesStatusType,
			ID:        optional.Some(runtime.DevicesID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VolumeManagerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *VolumeManagerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		// if devices are not ready, we can't provision volumes
		devicesStatus, err := safe.ReaderGetByID[*runtime.DevicesStatus](ctx, r, runtime.DevicesID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching devices status: %w", err)
		}

		devicesReady := devicesStatus != nil && devicesStatus.TypedSpec().Ready

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("error fetching discovered volumes: %w", err)
		}

		volumeConfigs, err := safe.ReaderListAll[*block.VolumeConfig](ctx, r)
		if err != nil {
			return fmt.Errorf("error fetching volume configurations: %w", err)
		}

		// ensure all volume configs have our finalizers
		for iter := volumeConfigs.Iterator(); iter.Next(); {
			vc := iter.Value()

			if vc.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if vc.Metadata().Finalizers().Has(ctrl.Name()) {
				continue
			}

			if err = r.AddFinalizer(ctx, vc.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer to volume configuration: %w", err)
			}
		}

		r.StartTrackingOutputs()

		// discovery phase
		volumeStatuses := map[string]*block.VolumeStatusSpec{}

		for iter := volumeConfigs.Iterator(); iter.Next(); {
			vc := iter.Value()

			if vc.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			volumeStatuses[vc.Metadata().ID()] = &block.VolumeStatusSpec{}

			if value.IsZero(vc.TypedSpec().Locator) {
				// can't be located, skip
				continue
			}

			for dvIter := discoveredVolumes.Iterator(); dvIter.Next(); {
				dv := dvIter.Value()

				if dv.Metadata().Phase() != resource.PhaseRunning {
					continue
				}

				if vc.TypedSpec().Locator.Matches(dv) {
					volumeStatuses[vc.Metadata().ID()].Located = true
					volumeStatuses[vc.Metadata().ID()].Location = dv.Metadata().ID()
				}
			}
		}

		// provision phase
		if devicesReady {
		}

		// update statuses
		for id, spec := range volumeStatuses {
			if err = safe.WriterModify(ctx, r, block.NewVolumeStatus(block.NamespaceName, id), func(vs *block.VolumeStatus) error {
				*vs.TypedSpec() = *spec

				return nil
			}); err != nil {
				return fmt.Errorf("error updating volume status: %w", err)
			}
		}

		// [TODO]: this would fail as it doesn't handle finalizers properly
		if err = safe.CleanupOutputs[*block.VolumeStatus](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up volume configuration: %w", err)
		}
	}
}
