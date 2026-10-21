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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/devsettle"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/md"
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

	// InactiveMDArrays lists assembled but inactive MD arrays, defaults to md.InactiveArrays.
	//
	// A degraded array stays inactive until storage.MDLastResortController force-runs it after a
	// grace period. Volumes on it (META/STATE on a mirrored system disk) must not be declared
	// missing before that, so DiscoveredVolumesStatus waits for inactive arrays to start.
	InactiveMDArrays func() ([]string, error)

	// MDArrayWaitTimeout bounds the wait for inactive MD arrays, defaults to DefaultMDArrayWaitTimeout.
	MDArrayWaitTimeout time.Duration
}

// DefaultMDArrayWaitTimeout is the default bound on waiting for inactive MD arrays to start.
//
// It exceeds the storage.MDLastResortController grace period (30s), so a degraded array is
// force-run before the volumes are located; an array which can't be run doesn't block the boot forever.
const DefaultMDArrayWaitTimeout = 45 * time.Second

const mdArrayPollInterval = 500 * time.Millisecond

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

			if err = ctrl.waitForMDArrays(ctx, logger); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}

				return fmt.Errorf("error waiting for the MD arrays to start: %w", err)
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

// waitForMDArrays waits (bounded) for inactive MD arrays to start, and then for their partitions to be enumerated.
func (ctrl *DiscoveredVolumesStatusController) waitForMDArrays(ctx context.Context, logger *zap.Logger) error {
	// MDLastResortController doesn't run in these modes, so inactive arrays would never start
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer || ctrl.V1Alpha1Mode.IsAgent() {
		return nil
	}

	inactiveArrays := ctrl.InactiveMDArrays
	if inactiveArrays == nil {
		inactiveArrays = md.InactiveArrays
	}

	timeout := ctrl.MDArrayWaitTimeout
	if timeout <= 0 {
		timeout = DefaultMDArrayWaitTimeout
	}

	started, err := pollInactiveMDArrays(ctx, logger, inactiveArrays, timeout)
	if err != nil || !started {
		return err
	}

	// the started arrays and their partitions are enumerated asynchronously
	return ctrl.waitForDevices(ctx, logger)
}

// pollInactiveMDArrays polls until there are no inactive MD arrays or the timeout expires.
//
// It returns true only if it had to wait and the arrays have started since.
func pollInactiveMDArrays(ctx context.Context, logger *zap.Logger, inactiveArrays func() ([]string, error), timeout time.Duration) (bool, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(mdArrayPollInterval)
	defer ticker.Stop()

	waited := false

	for {
		inactive, err := inactiveArrays()
		if err != nil {
			logger.Warn("failed to list MD arrays, not waiting for them", zap.Error(err))

			return waited, nil
		}

		if len(inactive) == 0 {
			return waited, nil
		}

		if !waited {
			logger.Info("waiting for inactive MD arrays to start", zap.Strings("arrays", inactive), zap.Duration("timeout", timeout))

			waited = true
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			logger.Warn("MD arrays are still inactive, proceeding without them", zap.Strings("arrays", inactive))

			return false, nil
		case <-ticker.C:
		}
	}
}
