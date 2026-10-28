// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMProvisioner is the reconciler's LVM subset.
type LVMProvisioner interface {
	PVCreate(ctx context.Context, device string) error
	VGCreate(ctx context.Context, vg string, pvs ...string) error
	VGExtend(ctx context.Context, vg string, pvs ...string) error
	VGChangeActivate(ctx context.Context, vg string) error
	VGChangeDeactivate(ctx context.Context, vg string) error
}

// LVMVolumeGroupReconcileController applies PV/VG state and activates the VGs declared in the machine config.
//
// Additive only. Destructive ops go through LVMService wipe RPCs.
//
// Parent volumes of a VG carry a finalizer which is released only after the VG
// is deactivated, so that e.g. LUKS is not closed under an active VG.
type LVMVolumeGroupReconcileController struct {
	V1Alpha1Mode machineruntime.Mode
	LVM          LVMProvisioner

	// vgActive records the latest activation state set for a VG.
	vgActive map[string]bool
}

// Name implements controller.Controller interface.
func (ctrl *LVMVolumeGroupReconcileController) Name() string {
	return "storage.LVMVolumeGroupReconcileController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMVolumeGroupReconcileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputStrong,
		},
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

	if ctrl.vgActive == nil {
		ctrl.vgActive = map[string]bool{}
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

		machineCfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("get machine config: %w", err)
		}

		parentsByVG := map[string][]string{}

		if machineCfg != nil {
			for _, doc := range machineCfg.Config().LVMVolumeGroupConfigs() {
				parentsByVG[doc.Name()] = doc.ParentVolumeIDs()
			}
		}

		volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("list VolumeStatus: %w", err)
		}

		volumeStatusByID := map[string]*block.VolumeStatus{}

		for vs := range volumeStatuses.All() {
			volumeStatusByID[vs.Metadata().ID()] = vs
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

		// A VG which is gone (e.g. its PVs were closed) should be activated again once it's back.
		for name := range ctrl.vgActive {
			if _, ok := observedVGByName[name]; !ok {
				delete(ctrl.vgActive, name)
			}
		}

		var reconcileErrs *multierror.Error

		// Parent volumes which should keep the finalizer.
		retainedParents := map[string]struct{}{}

		for spec := range vgSpecs.All() {
			name := spec.TypedSpec().Name

			parents := make([]*block.VolumeStatus, 0, len(parentsByVG[name]))

			for _, id := range parentsByVG[name] {
				if vs, ok := volumeStatusByID[id]; ok {
					parents = append(parents, vs)
				}
			}

			parentsReady := len(parents) == len(parentsByVG[name]) && !slices.ContainsFunc(parents, func(vs *block.VolumeStatus) bool {
				return vs.TypedSpec().Phase != block.VolumePhaseReady
			})

			if err := ctrl.reconcileSpec(
				ctx, r, logger, spec.TypedSpec(), parents, parentsReady, observedPVByDevice, observedVGByName, retainedParents,
			); err != nil {
				reconcileErrs = multierror.Append(reconcileErrs, fmt.Errorf("reconcile VG %q: %w", name, err))
			}
		}

		for vs := range volumeStatuses.All() {
			if _, ok := retainedParents[vs.Metadata().ID()]; ok || !vs.Metadata().Finalizers().Has(ctrl.Name()) {
				continue
			}

			if err := r.RemoveFinalizer(ctx, vs.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("remove finalizer from parent volume %q: %w", vs.Metadata().ID(), err)
			}
		}

		if err := reconcileErrs.ErrorOrNil(); err != nil {
			return fmt.Errorf("LVM reconcile encountered errors: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

// reconcileSpec converges one VG and its activation state, keeping the finalizers on
// the parent volumes recorded in retainedParents.
//
// Once any of the parents is torn down, the VG is deactivated and the finalizers are released.
func (ctrl *LVMVolumeGroupReconcileController) reconcileSpec(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	spec *storage.LVMVolumeGroupSpecSpec,
	parents []*block.VolumeStatus,
	parentsReady bool,
	observedPVByDevice map[string]*storage.LVMPhysicalVolumeStatus,
	observedVGByName map[string]*storage.LVMVolumeGroupStatus,
	retainedParents map[string]struct{},
) error {
	tearingDown := slices.ContainsFunc(parents, func(vs *block.VolumeStatus) bool {
		return vs.Metadata().Phase() == resource.PhaseTearingDown
	})

	if tearingDown {
		if err := ctrl.deactivateVG(ctx, logger, spec.Name, observedVGByName); err != nil {
			for _, vs := range parents {
				retainedParents[vs.Metadata().ID()] = struct{}{}
			}

			return fmt.Errorf("deactivate: %w", err)
		}

		return nil
	}

	for _, vs := range parents {
		retainedParents[vs.Metadata().ID()] = struct{}{}

		if !vs.Metadata().Finalizers().Has(ctrl.Name()) {
			if err := r.AddFinalizer(ctx, vs.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("add finalizer to parent volume %q: %w", vs.Metadata().ID(), err)
			}
		}
	}

	if !parentsReady {
		return nil
	}

	if err := ctrl.reconcileVG(ctx, logger, spec, observedPVByDevice, observedVGByName); err != nil {
		return err
	}

	return ctrl.activateVG(ctx, logger, spec.Name, observedVGByName)
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

// activateVG activates an assembled VG once, unless some of its PVs are missing.
func (ctrl *LVMVolumeGroupReconcileController) activateVG(
	ctx context.Context,
	logger *zap.Logger,
	name string,
	observedVGByName map[string]*storage.LVMVolumeGroupStatus,
) error {
	observedVG, ok := observedVGByName[name]
	if !ok || ctrl.vgActive[name] {
		return nil
	}

	if missing := observedVG.TypedSpec().MissingPVCount; missing != "" && missing != "0" {
		logger.Debug("waiting for missing LVM physical volumes", zap.String("vg", name), zap.String("missing", missing))

		return nil
	}

	logger.Info("activating LVM volume group", zap.String("vg", name))

	if err := ctrl.LVM.VGChangeActivate(ctx, name); err != nil && !errors.Is(err, exec.ErrNotFound) {
		return err
	}

	ctrl.vgActive[name] = true

	return nil
}

// deactivateVG deactivates a VG if it's present.
func (ctrl *LVMVolumeGroupReconcileController) deactivateVG(
	ctx context.Context,
	logger *zap.Logger,
	name string,
	observedVGByName map[string]*storage.LVMVolumeGroupStatus,
) error {
	if active, known := ctrl.vgActive[name]; known && !active {
		return nil
	}

	if _, ok := observedVGByName[name]; !ok {
		return nil
	}

	logger.Info("deactivating LVM volume group", zap.String("vg", name))

	if err := ctrl.LVM.VGChangeDeactivate(ctx, name); err != nil && !errors.Is(err, lvm.ErrNotFound) && !errors.Is(err, exec.ErrNotFound) {
		return err
	}

	ctrl.vgActive[name] = false

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
