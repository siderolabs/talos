// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// LVMActivationController activates LVM volume groups discovered by the block.DiscoveryController.
type LVMActivationController struct {
	V1Alpha1Mode machineruntime.Mode
	LVM          *lvm.LVM

	seenVolumes  map[string]struct{}
	activatedVGs map[string]struct{}
}

// Name implements controller.Controller interface.
func (ctrl *LVMActivationController) Name() string {
	return "storage.LVMActivationController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMActivationController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			ID:        optional.Some(constants.MetaPartitionLabel),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMActivationController) Outputs() []controller.Output {
	return nil
}

// preconditions ensures udevd is running and the META partition is mounted
// before scanning for LVM volume groups.
func (ctrl *LVMActivationController) preconditions(ctx context.Context, r controller.Reader, logger *zap.Logger) (bool, error) {
	udevdService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "udevd")
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get udevd service: %w", err)
	}

	if udevdService == nil {
		logger.Debug("udevd service not registered yet")

		return false, nil
	}

	if !udevdService.TypedSpec().Running || !udevdService.TypedSpec().Healthy {
		logger.Debug("waiting for udevd service to be running and healthy")

		return false, nil
	}

	meta, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, r, constants.MetaPartitionLabel)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get meta partition info: %w", err)
	}

	if meta == nil {
		logger.Debug("meta partition not registered yet")

		return false, nil
	}

	if meta.TypedSpec().Phase != block.VolumePhaseReady {
		logger.Debug("meta partition not ready yet")

		return false, nil
	}

	return true, nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LVMActivationController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.seenVolumes == nil {
		ctrl.seenVolumes = map[string]struct{}{}
	}

	if ctrl.activatedVGs == nil {
		ctrl.activatedVGs = map[string]struct{}{}
	}

	if ctrl.V1Alpha1Mode.IsAgent() {
		// in agent mode, we don't want to activate LVMs
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		ok, err := ctrl.preconditions(ctx, r, logger)
		if err != nil {
			return err
		}

		if !ok {
			continue
		}

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		var multiErr error

		for dv := range discoveredVolumes.All() {
			if dv.TypedSpec().Name != "lvm2-pv" {
				// if the volume is not an LVM volume the moment we saw it, we can skip it
				// we need to activate the volumes only on reboot, not when they are first formatted
				ctrl.seenVolumes[dv.Metadata().ID()] = struct{}{}

				continue
			}

			if _, ok := ctrl.seenVolumes[dv.Metadata().ID()]; ok {
				continue
			}

			logger.Debug("checking device for LVM volume activation", zap.String("device", dv.TypedSpec().DevPath))

			vgName, err := ctrl.checkVGNeedsActivation(ctx, dv.TypedSpec().DevPath)
			if err != nil {
				multiErr = multierror.Append(multiErr, err)

				continue
			}

			if vgName == "" {
				continue
			}

			if _, ok := ctrl.activatedVGs[vgName]; ok {
				continue
			}

			logger.Info("activating LVM volume", zap.String("name", vgName))

			if err = ctrl.LVM.VGChangeActivate(ctx, vgName); err != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to activate LVM volume %s: %w", vgName, err))
			} else {
				ctrl.activatedVGs[vgName] = struct{}{}
			}
		}

		if multiErr != nil {
			return multiErr
		}
	}
}

// checkVGNeedsActivation checks if the device is part of a complete volume
// group and returns that VG name when activation is needed; otherwise returns
// an empty string. See lvmautoactivation(7).
func (ctrl *LVMActivationController) checkVGNeedsActivation(ctx context.Context, devicePath string) (string, error) {
	udev, err := ctrl.LVM.PVScanAutoActivation(ctx, devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to check if LVM volume backed by device %s needs activation: %w", devicePath, err)
	}

	return udev[lvm.UdevKeyVGNameComplete], nil
}
