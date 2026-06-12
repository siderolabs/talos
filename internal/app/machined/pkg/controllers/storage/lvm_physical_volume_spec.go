// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"
	"sort"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/proto"
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

	volumes, err := buildVolumeProtos(ctx, r)
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
	volumes []volumeProto,
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
	volumes []volumeProto,
	claimedBy map[string]string,
	conflicts map[string]string,
) error {
	selector := doc.PhysicalVolumeSelector()

	for _, vol := range volumes {
		matches, err := selector.EvalBool(celenv.VolumeLocator(), vol.celContext)
		if err != nil {
			return fmt.Errorf("evaluate selector for VG %q: %w", doc.Name(), err)
		}

		if !matches {
			continue
		}

		// A partitioned whole disk can't be a PV; its partitions are matched
		// separately. Skip it so the reconciler doesn't try pvcreate on a
		// device that lvm rejects ("device is partitioned").
		if vol.partitioned {
			logger.Debug(
				"skipping partitioned disk as PV candidate; its partitions are preferred",
				zap.String("device", vol.devPath),
				zap.String("vg", doc.Name()),
			)

			continue
		}

		if prev, ok := claimedBy[vol.devPath]; ok && prev != doc.Name() {
			conflicts[doc.Name()] = fmt.Sprintf("device %q already claimed by volume group %q", vol.devPath, prev)

			logger.Warn(
				"disk claimed by multiple LVM volume groups; skipping",
				zap.String("device", vol.devPath),
				zap.String("first_vg", prev),
				zap.String("conflicting_vg", doc.Name()),
			)

			continue
		}

		claimedBy[vol.devPath] = doc.Name()

		if err := ctrl.writePVSpec(ctx, r, vol.devPath, doc.Name()); err != nil {
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

// volumeProto is a discovered volume prepared for CEL selector evaluation.
type volumeProto struct {
	devPath     string
	celContext  map[string]any
	partitioned bool
}

// buildVolumeProtos lists discovered volumes and prepares the CEL evaluation
// context for each. Every volume gets a `volume` variable. The `disk` variable
// is bound to the real disk only for whole-disk volumes; partitions get an
// empty DiskSpec so disk-level predicates (e.g. disk.transport == "nvme")
// evaluate to false against them rather than spanning the disk and all its
// partitions. Partitions are therefore selectable only via `volume.*`
// predicates (e.g. volume.partition_label), matching the documented contract.
//
//nolint:gocyclo
func buildVolumeProtos(ctx context.Context, r controller.Runtime) ([]volumeProto, error) {
	disks, err := safe.ReaderListAll[*block.Disk](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("list disks: %w", err)
	}

	diskByDevPath := map[string]*blockpb.DiskSpec{}

	for d := range disks.All() {
		spec := &blockpb.DiskSpec{}

		if err := proto.ResourceSpecToProto(d, spec); err != nil {
			return nil, fmt.Errorf("convert disk %q to proto: %w", d.Metadata().ID(), err)
		}

		diskByDevPath[spec.DevPath] = spec
	}

	volumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("list discovered volumes: %w", err)
	}

	// Devices that are parents of at least one partition. A partitioned
	// whole disk cannot back a PV directly.
	hasPartitions := map[string]struct{}{}

	for v := range volumes.All() {
		if parent := v.TypedSpec().ParentDevPath; parent != "" {
			hasPartitions[parent] = struct{}{}
		}
	}

	out := make([]volumeProto, 0, volumes.Len())

	for v := range volumes.All() {
		spec := &blockpb.DiscoveredVolumeSpec{}

		if err := proto.ResourceSpecToProto(v, spec); err != nil {
			return nil, fmt.Errorf("convert discovered volume %q to proto: %w", v.Metadata().ID(), err)
		}

		if spec.DevPath == "" {
			continue
		}

		// Bind the real disk only for whole-disk volumes (no parent). Partitions
		// and disks without a Disk resource get an empty DiskSpec so disk-level
		// predicates evaluate false instead of erroring on an unbound variable.
		disk := &blockpb.DiskSpec{}
		partitioned := false

		if spec.ParentDevPath == "" {
			if d, ok := diskByDevPath[spec.DevPath]; ok {
				disk = d
			}

			_, partitioned = hasPartitions[spec.DevPath]
		}

		celCtx := map[string]any{
			"volume": spec,
			"disk":   disk,
		}

		out = append(out, volumeProto{devPath: spec.DevPath, celContext: celCtx, partitioned: partitioned})
	}

	// Stable order for deterministic downstream iteration.
	sort.Slice(out, func(i, j int) bool { return out[i].devPath < out[j].devPath })

	return out, nil
}
