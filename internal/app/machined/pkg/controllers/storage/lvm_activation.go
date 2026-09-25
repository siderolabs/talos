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
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// LVMActivator is an interface used for testing.
type LVMActivator interface {
	PVScanAutoActivation(ctx context.Context, devicePath string) (map[string]string, error)
	VGChangeActivate(ctx context.Context, vgName string) error
}

// LVMActivationController activates LVM volume groups that Talos did not
// create itself - i.e. ones found pre-existing on disk, the same scope this
// controller had before declarative LVM provisioning was introduced.
//
// A VG backed by a LVMVolumeGroupConfig is left alone entirely:
// LVMVolumeGroupReconcileController owns its whole lifecycle (creation via
// vgcreate, which activates it as an ordinary side effect of the LVM tooling
// itself). This controller has no way to know what a foreign VG's backing
// devices actually are or what else might depend on them, so it doesn't
// place a finalizer on anything and doesn't participate in teardown -
// activation only, for volumes fully outside Talos's own declared state.
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
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMVolumeGroupSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMPhysicalVolumeSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMActivationController) Outputs() []controller.Output {
	return nil
}

// preconditions wait for udevd, META, and the machine config to be loaded.
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

	if _, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID); err != nil {
		if state.IsNotFoundError(err) {
			logger.Debug("machine config not loaded yet")

			return false, nil
		}

		return false, fmt.Errorf("failed to get machine config: %w", err)
	}

	return true, nil
}

// Run implements controller.Controller interface.
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

		if err := ctrl.reconcileNewActivations(ctx, r, logger); err != nil {
			return err
		}
	}
}

// reconcileNewActivations looks for complete LVM volume groups that Talos
// did not declare itself, and activates them if not already active.
func (ctrl *LVMActivationController) reconcileNewActivations(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	{
		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		pendingDevices, err := ctrl.pendingPVDevices(ctx, r)
		if err != nil {
			return err
		}

		managedVGs, err := ctrl.managedVGNames(ctx, r)
		if err != nil {
			return err
		}

		var multiErr error

		for dv := range discoveredVolumes.All() {
			_, pending := pendingDevices[dv.TypedSpec().DevPath]

			if pending {
				// This volume is pending to be provisioned as a PV - keep
				// re-checking it rather than writing it off as non-LVM.
				delete(ctrl.seenVolumes, dv.Metadata().ID())
			}

			if dv.TypedSpec().Name != "lvm2-pv" {
				if !pending {
					// Keep track of pre-existing non-LVM volumes and ones just formatted.
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

			if _, ok := managedVGs[vgName]; ok {
				// A VG managed by LVMVolumeGroupReconcileController.
				// ????Q: Deliberately not marked seen, so this keeps
				// getting rechecked - if the LVMVolumeGroupConfig is later
				// removed, this VG should fall back to being treated as foreign
				// rather than being left with nobody managing it.
				continue
			}

			if _, ok := ctrl.activatedVGs[vgName]; ok {
				continue
			}

			logger.Info("activating foreign LVM volume group", zap.String("name", vgName))

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

// managedVGNames returns the set of VG names declared via
// LVMVolumeGroupConfig (surfaced as LVMVolumeGroupSpec, one per doc,
// regardless of whether any PVs have matched it yet).
// LVMVolumeGroupReconcileController owns activation for these; this
// controller only ever activates VGs outside this set.
func (ctrl *LVMActivationController) managedVGNames(ctx context.Context, r controller.Reader) (map[string]struct{}, error) {
	vgSpecs, err := safe.ReaderListAll[*storage.LVMVolumeGroupSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list LVMVolumeGroupSpec: %w", err)
	}

	names := make(map[string]struct{}, vgSpecs.Len())

	for vg := range vgSpecs.All() {
		names[vg.TypedSpec().Name] = struct{}{}
	}

	return names, nil
}

// checkVGNeedsActivation returns VG name if auto-activation is needed.
func (ctrl *LVMActivationController) checkVGNeedsActivation(ctx context.Context, devicePath string) (string, error) {
	udev, err := ctrl.LVM.PVScanAutoActivation(ctx, devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to check if LVM volume backed by device %s needs activation: %w", devicePath, err)
	}

	return udev[lvm.UdevKeyVGNameComplete], nil
}
