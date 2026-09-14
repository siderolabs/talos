// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/devsettle"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// DiscoveredVolumesStatusController publishes DiscoveredVolumesStatus once devices are ready and volume discovery refresh is done.
type DiscoveredVolumesStatusController struct {
	V1Alpha1Mode machineruntime.Mode

	// WaitForDevices waits for the storage devices to be enumerated, defaults to devsettle.Settler.
	//
	// udevd settling is not enough: the kernel keeps enumerating devices asynchronously after
	// `udevadm settle` returns (USB bus scan, SD card initialization, NVMe and SCSI scans), so a
	// system disk on such a transport shows up a bit later. Without this wait the system disk is
	// not discovered before DiscoveredVolumesStatus goes ready, and the volume manager declares
	// META/STATE missing.
	WaitForDevices func(ctx context.Context, logger *zap.Logger) error
}

// Name implements controller.Controller interface.
func (ctrl *DiscoveredVolumesStatusController) Name() string {
	return "block.DiscoveredVolumesStatusController"
}

// Inputs implements controller.Controller interface.
func (ctl *DiscoveredVolumesStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.DevicesStatusType,
			ID:        optional.Some(runtime.DevicesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveryRefreshStatusType,
			ID:        optional.Some(block.RefreshID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DiscoveredVolumesStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.DiscoveredVolumesStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: block.DiscoveryRefreshRequestType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
// TODO(majabojarska): refactor to bring down cyclo
//
//nolint:gocyclo
func (ctrl *DiscoveredVolumesStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		devicesReadyObserved    bool
		discoveryRefreshRequest int
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// if devices are not ready, we can't provision and locate most volumes
		devicesStatus, err := safe.ReaderGetByID[*runtime.DevicesStatus](ctx, r, runtime.DevicesID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching devices status: %w", err)
		}

		devicesReady := devicesStatus != nil && devicesStatus.TypedSpec().Ready

		if devicesReady && !devicesReadyObserved {
			devicesReadyObserved = true

			// udevd is settled, but the kernel might still be enumerating storage devices, and
			// the system disk should be discovered before the volumes are declared missing
			if err = ctrl.waitForDevices(ctx, logger); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}

				return fmt.Errorf("error waiting for the storage devices to settle: %w", err)
			}

			// udevd reports that devices are ready, now it's time to refresh the discovery volumes
			if err = safe.WriterModify(ctx, r, block.NewDiscoveryRefreshRequest(block.NamespaceName, block.RefreshID), func(drr *block.DiscoveryRefreshRequest) error {
				drr.TypedSpec().Request++
				discoveryRefreshRequest = drr.TypedSpec().Request

				return nil
			}); err != nil {
				return fmt.Errorf("error updating discovery refresh request: %w", err)
			}
		}

		discoveryRefreshStatus, err := safe.ReaderGetByID[*block.DiscoveryRefreshStatus](ctx, r, block.RefreshID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching discovery refresh status: %w", err)
		}

		// now devicesReady is only true if the refresh status is up to date
		discoveredVolumesReady := devicesReady && discoveryRefreshStatus != nil && discoveryRefreshStatus.TypedSpec().Request == discoveryRefreshRequest

		if discoveredVolumesReady {
			if err = safe.WriterModify(ctx, r, block.NewDiscoveredVolumesStatus(block.NamespaceName, block.DiscoveredVolumesStatusID), func(dvr *block.DiscoveredVolumesStatus) error {
				dvr.TypedSpec().Ready = true

				return nil
			}); err != nil {
				return fmt.Errorf("error updating discovered volumes status: %w", err)
			}
		}
	}
}

func (ctrl *DiscoveredVolumesStatusController) waitForDevices(ctx context.Context, logger *zap.Logger) error {
	if ctrl.WaitForDevices != nil {
		return ctrl.WaitForDevices(ctx, logger)
	}

	// in container mode we don't own the devices, and sysfs is the host's one
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	return (&devsettle.Settler{}).Wait(ctx, logger)
}
