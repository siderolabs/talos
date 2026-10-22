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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

// LVMVolumeGroupReconcileController applies PV/VG state, and keeps managed
// VGs activated.
//
// Additive only in every other sense: existing PVs/VGs are left alone,
// nothing is resized or removed. Destructive ops go through LVMService wipe
// RPCs. Activation is the one exception - vgcreate activates a brand new
// VG's LVs as a side effect, but an already-existing VG (e.g. found already
// assembled across a reboot) is not activated by anything else, so this
// controller re-asserts activation on every reconcile regardless of whether
// it just created/extended the VG or found it already fully assembled.
//
// A VG whose provisioning.parents references Talos-managed RawVolume(s)
// (rather than a disk selector) additionally gets a finalizer placed on each
// backing volume's block.VolumeStatus, released only once that backing
// volume starts tearing down and this VG has been deactivated - keeping the
// RawVolume from closing out from under an active VG. A selector-matched VG
// gets no such finalizer: its backing devices are not exclusively owned by
// this VG's config the way a parents reference is.
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
			Type:      storage.LVMVolumeGroupStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMPhysicalVolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			// Strong: this controller must wake on every backing volume phase
			// transition to notice a parent volume starting to tear down, not
			// just ones it has already resolved into a PV.
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputStrong,
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

		machineCfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("get machine config: %w", err)
		}

		vgDocsByName := map[string]configconfig.LVMVolumeGroupConfig{}

		if machineCfg != nil {
			for _, doc := range machineCfg.Config().LVMVolumeGroupConfigs() {
				vgDocsByName[doc.Name()] = doc
			}
		}

		volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("list VolumeStatus: %w", err)
		}

		// Index observed PVs/VGs/backing volumes once per tick.
		observedPVByDevice := map[string]*storage.LVMPhysicalVolumeStatus{}

		for pv := range pvStatuses.All() {
			observedPVByDevice[pv.TypedSpec().Device] = pv
		}

		observedVGByName := map[string]*storage.LVMVolumeGroupStatus{}

		for vg := range vgStatuses.All() {
			observedVGByName[vg.TypedSpec().Name] = vg
		}

		volumeStatusByID := map[string]*block.VolumeStatus{}

		for vs := range volumeStatuses.All() {
			volumeStatusByID[vs.Metadata().ID()] = vs
		}

		var reconcileErrs *multierror.Error

		for spec := range vgSpecs.All() {
			vgName := spec.TypedSpec().Name

			if err := ctrl.reconcileVG(
				ctx, r, logger, vgDocsByName[vgName], spec.TypedSpec(), observedPVByDevice, observedVGByName, volumeStatusByID,
			); err != nil {
				reconcileErrs = multierror.Append(reconcileErrs, fmt.Errorf("reconcile VG %q: %w", vgName, err))
			}
		}

		if err := reconcileErrs.ErrorOrNil(); err != nil {
			return fmt.Errorf("LVM reconcile encountered errors: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

// reconcileVG converges one VG.
//
//nolint:gocyclo
func (ctrl *LVMVolumeGroupReconcileController) reconcileVG(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	doc configconfig.LVMVolumeGroupConfig,
	spec *storage.LVMVolumeGroupSpecSpec,
	observedPVByDevice map[string]*storage.LVMPhysicalVolumeStatus,
	observedVGByName map[string]*storage.LVMVolumeGroupStatus,
	volumeStatusByID map[string]*block.VolumeStatus,
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
	} else if missing := devicesMissingFromVG(spec.PhysicalVolumes, observedPVByDevice, observedVG.TypedSpec().Name); len(missing) > 0 {
		logger.Info(
			"extending LVM volume group",
			zap.String("vg", spec.Name),
			zap.Strings("devices", missing),
		)

		if err := ctrl.LVM.VGExtend(ctx, spec.Name, missing...); err != nil && !errors.Is(err, lvm.ErrExists) {
			return fmt.Errorf("vgextend %q: %w", spec.Name, err)
		}
	}

	if doc != nil && len(doc.Parents()) > 0 {
		return ctrl.reconcileParentVolumes(ctx, r, logger, spec.Name, doc.Parents(), volumeStatusByID)
	}

	return ctrl.ensureActive(ctx, logger, spec.Name)
}

// reconcileParentVolumes keeps the finalizers on a parents-based VG's backing
// RawVolume(s) up to date, and deactivates the VG once any of them starts
// tearing down, releasing the finalizers so the teardown can proceed.
//
// Unlike LVMActivationController (which only ever sees bare devices, having
// no config doc for a foreign VG to work from), a parents-based VG's config
// names its backing volumes directly, so the block.VolumeStatus resources
// are looked up by id (the RawVolume kind's id prefix plus its name) rather
// than by reverse-matching device paths.
func (ctrl *LVMVolumeGroupReconcileController) reconcileParentVolumes(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	vgName string,
	parents []configconfig.ProvisioningVolumeParent,
	volumeStatusByID map[string]*block.VolumeStatus,
) error {
	finalizer := ctrl.Name() + "-" + vgName

	backing := make([]*block.VolumeStatus, 0, len(parents))

	for _, parent := range parents {
		if parent.Kind != "RawVolume" {
			continue
		}

		if vs, ok := volumeStatusByID[constants.RawVolumePrefix+parent.Name]; ok {
			backing = append(backing, vs)
		}
	}

	tearingDown := false

	for _, vs := range backing {
		if vs.Metadata().Phase() == resource.PhaseTearingDown {
			tearingDown = true

			break
		}
	}

	if tearingDown {
		logger.Info("deactivating LVM volume group, parent volume tearing down", zap.String("vg", vgName))

		if err := ctrl.LVM.VGChangeDeactivate(ctx, vgName); err != nil && !errors.Is(err, lvm.ErrNotFound) {
			return fmt.Errorf("deactivate vg %q: %w", vgName, err)
		}

		for _, vs := range backing {
			if !vs.Metadata().Finalizers().Has(finalizer) {
				continue
			}

			if err := r.RemoveFinalizer(ctx, vs.Metadata(), finalizer); err != nil {
				return fmt.Errorf("remove finalizer from parent volume %q: %w", vs.Metadata().ID(), err)
			}
		}

		return nil
	}

	for _, vs := range backing {
		if vs.Metadata().Finalizers().Has(finalizer) {
			continue
		}

		if err := r.AddFinalizer(ctx, vs.Metadata(), finalizer); err != nil {
			return fmt.Errorf("add finalizer to parent volume %q: %w", vs.Metadata().ID(), err)
		}
	}

	return ctrl.ensureActive(ctx, logger, vgName)
}

// ensureActive activates the VG via the same vgchange -aay --autoactivation
// event call LVMActivationController uses for foreign VGs. vgcreate
// activates a brand new VG's LVs as a side effect, but nothing else brings
// a Talos-managed VG back up across a reboot: when the VG/PVs already exist
// exactly as declared, nothing above this point in reconcileVG is
// "missing", so this is the only step that runs at all in that case.
//
// Called unconditionally on every reconcile (idempotent, and cheap once
// steady state is reached - vgchange -aay is a no-op when everything
// eligible is already active), rather than gated on some remembered
// "already activated" state, so a VG that goes inactive for any reason
// (e.g. a foreign tool deactivated it) gets reactivated on the next event
// too.
func (ctrl *LVMVolumeGroupReconcileController) ensureActive(ctx context.Context, logger *zap.Logger, vgName string) error {
	if err := ctrl.LVM.VGChangeActivate(ctx, vgName); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			logger.Warn("lvm binary not found; skipping LVM provisioning")

			return nil
		}

		return fmt.Errorf("activate vg %q: %w", vgName, err)
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
