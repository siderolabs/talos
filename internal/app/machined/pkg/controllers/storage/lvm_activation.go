// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// LVMActivator is an interface used for testing.
type LVMActivator interface {
	PVScanAutoActivation(ctx context.Context, devicePath string) (map[string]string, error)
	VGChangeActivate(ctx context.Context, vgName string) error
	VGChangeDeactivate(ctx context.Context, vgName string) error
}

// LVMActivationController activates discovered LVM volume groups.
type LVMActivationController struct {
	V1Alpha1Mode machineruntime.Mode
	LVM          LVMActivator

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
			Kind:      controller.InputStrong,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMPhysicalVolumeSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMPhysicalVolumeStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMActivationController) Outputs() []controller.Output {
	return nil
}

// preconditions wait for udevd and META. These are required to activate new VGs -
// Code paths to track ones already activated runs unconditionally.
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
//nolint:gocyclo
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

		var errs *multierror.Error

		ok, err := ctrl.preconditions(ctx, r, logger)
		if err != nil {
			errs = multierror.Append(errs, err)
		} else if ok {
			if err := ctrl.reconcileNewActivations(ctx, r, logger); err != nil {
				errs = multierror.Append(errs, err)
			}
		}

		// Claim active PVs to handle teardown - do it regardless of preconditions
		// to deactivate any activated VG if it ever is activated.
		if err := ctrl.reconcileActivated(ctx, r, logger); err != nil {
			errs = multierror.Append(errs, err)
		}

		if err := errs.ErrorOrNil(); err != nil {
			return err
		}
	}
}

// reconcileNewActivations looks for LVM volume groups that are complete and
// not yet activated, and activates them.
//
//nolint:gocyclo
func (ctrl *LVMActivationController) reconcileNewActivations(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list discovered volumes: %w", err)
	}

	pendingDevices, err := ctrl.pendingPVDevices(ctx, r)
	if err != nil {
		return err
	}

	var multiErr error

	for dv := range discoveredVolumes.All() {
		_, pending := pendingDevices[dv.TypedSpec().DevPath]

		if pending {
			// This volume is pending to be provisioned as a PV - then
			// we must track it with a finalizer.
			delete(ctrl.seenVolumes, dv.Metadata().ID())
		}

		if dv.TypedSpec().Name != "lvm2-pv" {
			if !pending {
				// Keep track of pre-existing LVM volumes and ones just formatted.
				ctrl.seenVolumes[dv.Metadata().ID()] = struct{}{}
			}

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

	return nil
}

// pendingPVDevices returns the set of device paths claimed by any current
// storage.LVMPhysicalVolumeSpec, including current and pending PVs.
func (ctrl *LVMActivationController) pendingPVDevices(ctx context.Context, r controller.Reader) (map[string]struct{}, error) {
	pvSpecs, err := safe.ReaderListAll[*storage.LVMPhysicalVolumeSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list LVMPhysicalVolumeSpec: %w", err)
	}

	devices := make(map[string]struct{})

	for pv := range pvSpecs.All() {
		devices[pv.TypedSpec().Device] = struct{}{}
	}

	return devices, nil
}

// reconcileActivated maintains finalizers on all the active VGs, and
// deactivates VGs when backing PVs are tearing down.
//
//nolint:gocyclo
func (ctrl *LVMActivationController) reconcileActivated(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if len(ctrl.activatedVGs) == 0 {
		return nil
	}

	pvStatuses, err := safe.ReaderListAll[*storage.LVMPhysicalVolumeStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("list LVMPhysicalVolumeStatus: %w", err)
	}

	devicesByVG := map[string][]string{}

	for pv := range pvStatuses.All() {
		spec := pv.TypedSpec()
		devicesByVG[spec.VGName] = append(devicesByVG[spec.VGName], spec.Device)
	}

	volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("list VolumeStatus: %w", err)
	}

	volumeStatusByLocation := map[string]*block.VolumeStatus{}

	for vs := range volumeStatuses.All() {
		spec := vs.TypedSpec()

		switch {
		case spec.MountLocation != "":
			volumeStatusByLocation[resolveDevicePath(spec.MountLocation)] = vs
		case spec.Location != "":
			volumeStatusByLocation[resolveDevicePath(spec.Location)] = vs
		}
	}

	var errs *multierror.Error

	for vgName := range ctrl.activatedVGs {
		backingVolumes := backingRawVolumes(devicesByVG[vgName], volumeStatusByLocation)
		finalizer := ctrl.Name() + "-" + vgName

		tearingDown := false

		for _, vs := range backingVolumes {
			if vs.Metadata().Phase() == resource.PhaseTearingDown {
				tearingDown = true

				break
			}
		}

		if tearingDown {
			if err := ctrl.teardownVG(ctx, logger, r, vgName, finalizer, backingVolumes); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("teardown vg %q: %w", vgName, err))
			}

			continue
		}

		for _, vs := range backingVolumes {
			if vs.TypedSpec().Phase != block.VolumePhaseReady {
				continue
			}

			if vs.Metadata().Finalizers().Has(finalizer) {
				continue
			}

			if err := r.AddFinalizer(ctx, vs.Metadata(), finalizer); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("add finalizer to volume %q: %w", vs.Metadata().ID(), err))
			}
		}
	}

	return errs.ErrorOrNil()
}

// teardownVG deactivates a VG whose backing raw volume(s) are tearing down,
// then releases the finalizers related to that VG resuming the teardown (if present).
//
// Try to deactivate even if backing volumes do not have the finalizer to prevent
// race conditions when deactivation happens before this controller's loop.
func (ctrl *LVMActivationController) teardownVG(
	ctx context.Context,
	logger *zap.Logger,
	r controller.Runtime,
	vgName string,
	finalizer string,
	backingVolumes []*block.VolumeStatus,
) error {
	logger.Info("deactivating LVM volume group", zap.String("vg", vgName))

	// Ignore errors if the VG is already deactivated by other means
	if err := ctrl.LVM.VGChangeDeactivate(ctx, vgName); err != nil &&
		!errors.Is(err, lvm.ErrNotFound) {
		return fmt.Errorf("deactivate vg %q: %w", vgName, err)
	}

	for _, vs := range backingVolumes {
		if !vs.Metadata().Finalizers().Has(finalizer) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, vs.Metadata(), finalizer); err != nil {
			return fmt.Errorf("remove finalizer from volume %q: %w", vs.Metadata().ID(), err)
		}
	}

	delete(ctrl.activatedVGs, vgName)

	return nil
}

// backingRawVolumes finds the set of VolumeStatus resources backing the
// given device paths (excluding non-volume ones) - PV paths.
func backingRawVolumes(devices []string, byLocation map[string]*block.VolumeStatus) []*block.VolumeStatus {
	seen := map[string]struct{}{}
	result := make([]*block.VolumeStatus, 0, len(devices))

	for _, device := range devices {
		vs, ok := byLocation[resolveDevicePath(device)]
		if !ok {
			continue
		}

		id := vs.Metadata().ID()
		if _, dup := seen[id]; dup {
			continue
		}

		seen[id] = struct{}{}

		result = append(result, vs)
	}

	return result
}

// resolveDevicePath recursively resolves symlinks for key/comparison of device paths.
// This enables aliases to be correctly identified.
func resolveDevicePath(path string) string {
	if path == "" {
		return ""
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}

	return resolved
}

// checkVGNeedsActivation returns VG name if auto-activation is needed.
func (ctrl *LVMActivationController) checkVGNeedsActivation(ctx context.Context, devicePath string) (string, error) {
	udev, err := ctrl.LVM.PVScanAutoActivation(ctx, devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to check if LVM volume backed by device %s needs activation: %w", devicePath, err)
	}

	return udev[lvm.UdevKeyVGNameComplete], nil
}
