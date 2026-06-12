// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMProvisioner is the reconciler's LVM subset.
type LVMProvisioner interface {
	PVCreate(ctx context.Context, device string) error
	VGCreate(ctx context.Context, vg string, pvs ...string) error
	VGExtend(ctx context.Context, vg string, pvs ...string) error
}

// LVMVolumeGroupReconcileController applies PV/VG state.
//
// Additive only. Destructive ops go through LVMService wipe RPCs.
type LVMVolumeGroupReconcileController struct {
	V1Alpha1Mode machineruntime.Mode
	LVM          LVMProvisioner
}

// Name implements controller.Controller interface.
func (ctrl *LVMVolumeGroupReconcileController) Name() string {
	return "storage.LVMVolumeGroupReconcileController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMVolumeGroupReconcileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMVolumeGroupSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMVolumeGroupStatusType,
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
func (ctrl *LVMVolumeGroupReconcileController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LVMVolumeGroupReconcileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// in container mode, no devices, nothing to provision
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	if ctrl.LVM == nil {
		return errors.New("LVM provisioner not configured")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		vgSpecs, err := safe.ReaderListAll[*storage.LVMVolumeGroupSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMVolumeGroupSpec: %w", err)
		}

		if vgSpecs.Len() == 0 {
			continue
		}

		pvStatuses, err := safe.ReaderListAll[*storage.LVMPhysicalVolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMPhysicalVolumeStatus: %w", err)
		}

		vgStatuses, err := safe.ReaderListAll[*storage.LVMVolumeGroupStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMVolumeGroupStatus: %w", err)
		}

		// Index observed PVs/VGs once per tick.
		observedPVByDevice := map[string]*storage.LVMPhysicalVolumeStatus{}

		for pv := range pvStatuses.All() {
			observedPVByDevice[pv.TypedSpec().Device] = pv
		}

		observedVGByName := map[string]*storage.LVMVolumeGroupStatus{}

		for vg := range vgStatuses.All() {
			observedVGByName[vg.TypedSpec().Name] = vg
		}

		var reconcileErrs *multierror.Error

		for spec := range vgSpecs.All() {
			if err := ctrl.reconcileVG(ctx, logger, spec.TypedSpec(), observedPVByDevice, observedVGByName); err != nil {
				reconcileErrs = multierror.Append(reconcileErrs, fmt.Errorf("reconcile VG %q: %w", spec.TypedSpec().Name, err))
			}
		}

		if err := reconcileErrs.ErrorOrNil(); err != nil {
			// Log and retry on next event.
			logger.Warn("LVM reconcile encountered errors", zap.Error(err))
		}
	}
}

// reconcileVG converges one VG.
//
//nolint:gocyclo
func (ctrl *LVMVolumeGroupReconcileController) reconcileVG(
	ctx context.Context,
	logger *zap.Logger,
	spec *storage.LVMVolumeGroupSpecSpec,
	observedPVByDevice map[string]*storage.LVMPhysicalVolumeStatus,
	observedVGByName map[string]*storage.LVMVolumeGroupStatus,
) error {
	if len(spec.PhysicalVolumes) == 0 {
		return nil
	}

	for _, device := range spec.PhysicalVolumes {
		if _, ok := observedPVByDevice[device]; ok {
			continue
		}

		logger.Info("creating LVM physical volume", zap.String("device", device), zap.String("vg", spec.Name))

		if err := ctrl.LVM.PVCreate(ctx, device); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				logger.Warn("lvm binary not found; skipping LVM provisioning")

				return nil
			}

			// Idempotent: the device may already be a PV (status scan lag).
			if errors.Is(err, lvm.ErrExists) {
				continue
			}

			return fmt.Errorf("pvcreate %q: %w", device, err)
		}
	}

	observedVG, vgExists := observedVGByName[spec.Name]

	if !vgExists {
		logger.Info(
			"creating LVM volume group",
			zap.String("vg", spec.Name),
			zap.Strings("devices", spec.PhysicalVolumes),
		)

		if err := ctrl.LVM.VGCreate(ctx, spec.Name, spec.PhysicalVolumes...); err != nil && !errors.Is(err, lvm.ErrExists) {
			return fmt.Errorf("vgcreate %q: %w", spec.Name, err)
		}

		return nil
	}

	missing := devicesMissingFromVG(spec.PhysicalVolumes, observedPVByDevice, observedVG.TypedSpec().Name)
	if len(missing) == 0 {
		return nil
	}

	logger.Info(
		"extending LVM volume group",
		zap.String("vg", spec.Name),
		zap.Strings("devices", missing),
	)

	if err := ctrl.LVM.VGExtend(ctx, spec.Name, missing...); err != nil && !errors.Is(err, lvm.ErrExists) {
		return fmt.Errorf("vgextend %q: %w", spec.Name, err)
	}

	return nil
}

// devicesMissingFromVG returns desired devices not yet in target VG.
func devicesMissingFromVG(
	desired []string,
	observedPVByDevice map[string]*storage.LVMPhysicalVolumeStatus,
	vgName string,
) []string {
	var missing []string

	for _, device := range desired {
		pv, ok := observedPVByDevice[device]
		if !ok {
			missing = append(missing, device)

			continue
		}

		if pv.TypedSpec().VGName != vgName {
			missing = append(missing, device)
		}
	}

	return missing
}
