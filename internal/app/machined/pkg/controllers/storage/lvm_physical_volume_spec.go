// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block/blockhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMPhysicalVolumeSpecController evaluates v1alpha1 LVMVolumeGroupConfig
// selectors against discovered volumes (whole disks and partitions) and emits
// one LVMPhysicalVolumeSpec per match.
type LVMPhysicalVolumeSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *LVMPhysicalVolumeSpecController) Name() string {
	return "storage.LVMPhysicalVolumeSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMPhysicalVolumeSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
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
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMPhysicalVolumeSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.LVMPhysicalVolumeSpecType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: storage.LVMValidationErrorType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *LVMPhysicalVolumeSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}
	}
}

// reconcile runs a single reconciliation pass.
func (ctrl *LVMPhysicalVolumeSpecController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	machineCfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("get machine config: %w", err)
	}

	var vgDocs []configconfig.LVMVolumeGroupConfig

	if machineCfg != nil {
		vgDocs = machineCfg.Config().LVMVolumeGroupConfigs()
	}

	volumes, err := buildMatchContexts(ctx, r)
	if err != nil {
		return err
	}

	r.StartTrackingOutputs()

	if err := ctrl.emitSpecs(ctx, r, logger, vgDocs, volumes); err != nil {
		return err
	}

	if err := r.CleanupOutputs(
		ctx,
		resource.NewMetadata(storage.NamespaceName, storage.LVMPhysicalVolumeSpecType, "", resource.VersionUndefined),
		resource.NewMetadata(storage.NamespaceName, storage.LVMValidationErrorType, "", resource.VersionUndefined),
	); err != nil {
		return fmt.Errorf("cleanup outputs: %w", err)
	}

	return nil
}

// emitSpecs evaluates every VG selector against the discovered volumes and
// writes PV specs for matches, recording overlap conflicts as validation
// errors.
func (ctrl *LVMPhysicalVolumeSpecController) emitSpecs(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	vgDocs []configconfig.LVMVolumeGroupConfig,
	volumes []blockhelpers.MatchContext,
) error {
	// Per-device claim map: detect VGs whose selectors overlap (LVM forbids a
	// PV in two VGs).
	claimedBy := map[string]string{}

	// Conflicts recorded per losing VG, surfaced as LVMValidationError.
	conflicts := map[string]string{}

	for _, doc := range vgDocs {
		if doc.PhysicalVolumeSelector().IsZero() {
			continue
		}

		if err := ctrl.matchVolumesToVG(ctx, r, logger, doc, volumes, claimedBy, conflicts); err != nil {
			return err
		}
	}

	for vgName, msg := range conflicts {
		if err := ctrl.writeValidationError(ctx, r, vgName, msg); err != nil {
			return err
		}
	}

	return nil
}

// matchVolumesToVG evaluates the selector of a single VG doc against all
// volumes, updating claimedBy for matches and conflicts for overlaps.
func (ctrl *LVMPhysicalVolumeSpecController) matchVolumesToVG(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	doc configconfig.LVMVolumeGroupConfig,
	volumes []blockhelpers.MatchContext,
	claimedBy map[string]string,
	conflicts map[string]string,
) error {
	selector := doc.PhysicalVolumeSelector()

	for _, vol := range volumes {
		matches, err := selector.EvalBool(celenv.VolumeLocator(), vol.CELContext)
		if err != nil {
			return fmt.Errorf("evaluate selector for VG %q: %w", doc.Name(), err)
		}

		if !matches {
			continue
		}

		// A partitioned whole disk can't be a PV; its partitions are matched
		// separately. Skip it so the reconciler doesn't try pvcreate on a
		// device that lvm rejects ("device is partitioned").
		if vol.Partitioned {
			logger.Debug(
				"skipping partitioned disk as PV candidate; its partitions are preferred",
				zap.String("device", vol.DevPath),
				zap.String("vg", doc.Name()),
			)

			continue
		}

		if prev, ok := claimedBy[vol.DevPath]; ok && prev != doc.Name() {
			conflicts[doc.Name()] = fmt.Sprintf("device %q already claimed by volume group %q", vol.DevPath, prev)

			logger.Warn(
				"disk claimed by multiple LVM volume groups; skipping",
				zap.String("device", vol.DevPath),
				zap.String("first_vg", prev),
				zap.String("conflicting_vg", doc.Name()),
			)

			continue
		}

		claimedBy[vol.DevPath] = doc.Name()

		if err := ctrl.writePVSpec(ctx, r, vol.DevPath, doc.Name()); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *LVMPhysicalVolumeSpecController) writePVSpec(ctx context.Context, r controller.Runtime, devPath, vgName string) error {
	id := pvID(devPath)

	if err := safe.WriterModify(
		ctx, r,
		storage.NewLVMPhysicalVolumeSpec(storage.NamespaceName, id),
		func(s *storage.LVMPhysicalVolumeSpec) error {
			pvSpec := s.TypedSpec()
			pvSpec.Device = devPath
			pvSpec.VGName = vgName

			return nil
		},
	); err != nil {
		return fmt.Errorf("modify LVMPhysicalVolumeSpec %q: %w", id, err)
	}

	return nil
}

func (ctrl *LVMPhysicalVolumeSpecController) writeValidationError(ctx context.Context, r controller.Runtime, vgName, message string) error {
	if err := safe.WriterModify(
		ctx, r,
		storage.NewLVMValidationError(storage.NamespaceName, vgName),
		func(e *storage.LVMValidationError) error {
			spec := e.TypedSpec()
			spec.VGName = vgName
			spec.Message = message

			return nil
		},
	); err != nil {
		return fmt.Errorf("modify LVMValidationError %q: %w", vgName, err)
	}

	return nil
}

// buildMatchContexts lists discovered volumes, disks and the system disk, and
// delegates CEL context construction to blockhelpers.BuildMatchContexts. Every
// volume gets a `volume` variable; `disk` is bound to the real disk only for
// whole-disk volumes so disk-level predicates (e.g. disk.transport == "nvme")
// evaluate false against partitions rather than spanning the disk and all its
// partitions. Partitions are therefore selectable only via `volume.*`
// predicates (e.g. volume.partition_label), matching the documented contract.
func buildMatchContexts(ctx context.Context, r controller.Runtime) ([]blockhelpers.MatchContext, error) {
	disks, err := safe.ReaderListAll[*block.Disk](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("list disks: %w", err)
	}

	volumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("list discovered volumes: %w", err)
	}

	systemDiskDevPath := ""

	systemDisk, err := safe.ReaderGetByID[*block.SystemDisk](ctx, r, block.SystemDiskID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("get system disk: %w", err)
	}

	if systemDisk != nil {
		systemDiskDevPath = systemDisk.TypedSpec().DevPath
	}

	return blockhelpers.BuildMatchContexts(slices.Collect(disks.All()), slices.Collect(volumes.All()), systemDiskDevPath)
}
