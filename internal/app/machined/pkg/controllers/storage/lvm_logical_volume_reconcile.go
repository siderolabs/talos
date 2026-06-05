// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// parseSizeBytes parses an LVM raw byte-size column, returning 0 when the value
// is empty or unparseable (treated as "unknown", which never triggers a grow).
func parseSizeBytes(raw string) uint64 {
	size, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}

	return size
}

// LVMLogicalVolumeProvisioner is the reconciler's LVM subset.
type LVMLogicalVolumeProvisioner interface {
	LVCreate(ctx context.Context, vg, lv string, opts lvm.LVCreateOptions) error
	LVExtend(ctx context.Context, vg, lv string, opts lvm.LVExtendOptions) error
}

// LVMLogicalVolumeReconcileController creates logical volumes from
// LVMLogicalVolumeSpec.
//
// Additive only: existing LVs are left alone, none are resized or removed.
// Destructive ops go through the LVMService LV remove RPC.
type LVMLogicalVolumeReconcileController struct {
	V1Alpha1Mode machineruntime.Mode
	LVM          LVMLogicalVolumeProvisioner
}

// Name implements controller.Controller interface.
func (ctrl *LVMLogicalVolumeReconcileController) Name() string {
	return "storage.LVMLogicalVolumeReconcileController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMLogicalVolumeReconcileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMLogicalVolumeSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMVolumeGroupStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMLogicalVolumeStatusType,
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
func (ctrl *LVMLogicalVolumeReconcileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.LVMValidationErrorType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LVMLogicalVolumeReconcileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

		lvSpecs, err := safe.ReaderListAll[*storage.LVMLogicalVolumeSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMLogicalVolumeSpec: %w", err)
		}

		if lvSpecs.Len() == 0 {
			continue
		}

		// VGs present on disk; LVs can only be created in an assembled VG.
		vgPresent := map[string]struct{}{}

		vgStatuses, err := safe.ReaderListAll[*storage.LVMVolumeGroupStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMVolumeGroupStatus: %w", err)
		}

		// VG total size in bytes, used to compute the byte target of
		// percentage-sized LVs when deciding whether to grow.
		vgSizeBytes := map[string]uint64{}

		for vg := range vgStatuses.All() {
			vgPresent[vg.TypedSpec().Name] = struct{}{}
			vgSizeBytes[vg.TypedSpec().Name] = parseSizeBytes(vg.TypedSpec().Size)
		}

		// Existing LVs keyed by lvID(vg/lv) -> observed size in bytes, so
		// reconciliation is idempotent and can detect grow opportunities. A
		// size that fails to parse is recorded as 0 (treated as "unknown",
		// never triggers a grow).
		lvObservedSize := map[string]uint64{}

		lvStatuses, err := safe.ReaderListAll[*storage.LVMLogicalVolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMLogicalVolumeStatus: %w", err)
		}

		for lv := range lvStatuses.All() {
			key := lv.TypedSpec().FullName
			if key == "" {
				key = lv.TypedSpec().Path
			}

			lvObservedSize[lvID(key)] = parseSizeBytes(lv.TypedSpec().Size)
		}

		// PV counts per VG, for the raid1 minimum-members gate.
		pvCountByVG := map[string]int{}

		pvStatuses, err := safe.ReaderListAll[*storage.LVMPhysicalVolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMPhysicalVolumeStatus: %w", err)
		}

		for pv := range pvStatuses.All() {
			pvCountByVG[pv.TypedSpec().VGName]++
		}

		r.StartTrackingOutputs()

		var reconcileErrs *multierror.Error

		for spec := range lvSpecs.All() {
			validationMsg, err := ctrl.reconcileLV(ctx, logger, spec.TypedSpec(), vgPresent, vgSizeBytes, lvObservedSize, pvCountByVG)
			if err != nil {
				reconcileErrs = multierror.Append(reconcileErrs, fmt.Errorf("reconcile LV %q/%q: %w", spec.TypedSpec().VGName, spec.TypedSpec().Name, err))
			}

			if validationMsg != "" {
				if werr := ctrl.writeValidationError(ctx, r, spec.TypedSpec(), validationMsg); werr != nil {
					reconcileErrs = multierror.Append(reconcileErrs, werr)
				}
			}
		}

		if err := safe.CleanupOutputs[*storage.LVMValidationError](ctx, r); err != nil {
			return fmt.Errorf("cleanup LVMValidationError outputs: %w", err)
		}

		if err := reconcileErrs.ErrorOrNil(); err != nil {
			// Log and retry on next event.
			logger.Warn("LVM logical volume reconcile encountered errors", zap.Error(err))
		}
	}
}

// writeValidationError surfaces an unsupported reconciliation request (e.g. a
// requested shrink) for the logical volume as an LVMValidationError.
func (ctrl *LVMLogicalVolumeReconcileController) writeValidationError(
	ctx context.Context,
	r controller.Runtime,
	spec *storage.LVMLogicalVolumeSpecSpec,
	message string,
) error {
	id := spec.VGName + "/" + spec.Name

	if err := safe.WriterModify(
		ctx, r,
		storage.NewLVMValidationError(storage.NamespaceName, id),
		func(e *storage.LVMValidationError) error {
			e.TypedSpec().VGName = spec.VGName
			e.TypedSpec().Message = message

			return nil
		},
	); err != nil {
		return fmt.Errorf("modify LVMValidationError %q: %w", id, err)
	}

	return nil
}

// reconcileLV creates one LV if its VG is ready and the LV does not yet exist,
// or resizes it towards the desired size. The returned string, when non-empty,
// is a validation message to surface (e.g. an unsupported shrink request).
func (ctrl *LVMLogicalVolumeReconcileController) reconcileLV(
	ctx context.Context,
	logger *zap.Logger,
	spec *storage.LVMLogicalVolumeSpecSpec,
	vgPresent map[string]struct{},
	vgSizeBytes map[string]uint64,
	lvObservedSize map[string]uint64,
	pvCountByVG map[string]int,
) (string, error) {
	if _, ok := vgPresent[spec.VGName]; !ok {
		// VG not assembled yet; wait for a later event.
		return "", nil
	}

	if observed, ok := lvObservedSize[lvID(spec.VGName+"/"+spec.Name)]; ok {
		return ctrl.maybeResizeLV(ctx, logger, spec, vgSizeBytes[spec.VGName], observed)
	}

	pvCount := pvCountByVG[spec.VGName]

	mirrors, stripes, ok := resolveRAIDParams(spec, pvCount)
	if !ok {
		logger.Warn(
			"skipping logical volume: volume group has too few physical volumes for the requested layout",
			zap.String("vg", spec.VGName),
			zap.String("lv", spec.Name),
			zap.Stringer("type", spec.Type),
			zap.Int("pv_count", pvCount),
		)

		return "", nil
	}

	logger.Info(
		"creating LVM logical volume",
		zap.String("vg", spec.VGName),
		zap.String("lv", spec.Name),
		zap.Stringer("type", spec.Type),
		zap.Uint32("mirrors", mirrors),
		zap.Uint32("stripes", stripes),
	)

	if err := ctrl.LVM.LVCreate(ctx, spec.VGName, spec.Name, lvm.LVCreateOptions{
		Type:          spec.Type.String(),
		Mirrors:       mirrors,
		Stripes:       stripes,
		SizeBytes:     spec.SizeBytes,
		SizePercentVG: spec.SizePercentVG,
	}); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			logger.Warn("lvm binary not found; skipping LVM provisioning")

			return "", nil
		}

		return "", fmt.Errorf("lvcreate: %w", err)
	}

	return "", nil
}

// resolveRAIDParams resolves the mirror/stripe counts for the LV's layout
// against the number of PVs in the VG, auto-filling stripes (0 = "all PVs")
// where requested. It returns ok=false when the VG has too few PVs for the
// layout, signaling the caller to skip (and retry later).
//
//nolint:gocyclo
func resolveRAIDParams(spec *storage.LVMLogicalVolumeSpecSpec, pvCount int) (mirrors, stripes uint32, ok bool) {
	switch spec.Type {
	case storage.LVMLogicalVolumeTypeLinear:
		return 0, 0, true
	case storage.LVMLogicalVolumeTypeRAID0:
		stripes = spec.Stripes
		if stripes == 0 {
			stripes = uint32(pvCount)
		}

		return 0, stripes, stripes >= 2 && pvCount >= int(stripes)
	case storage.LVMLogicalVolumeTypeRAID1:
		mirrors = spec.Mirrors
		if mirrors == 0 {
			mirrors = 1
		}

		return mirrors, 0, pvCount >= int(mirrors)+1
	case storage.LVMLogicalVolumeTypeRAID10:
		mirrors = spec.Mirrors
		if mirrors == 0 {
			mirrors = 1
		}

		stripes = spec.Stripes
		if stripes == 0 {
			stripes = uint32(pvCount) / (mirrors + 1)
		}

		return mirrors, stripes, stripes >= 2 && pvCount >= int(stripes)*int(mirrors+1)
	default:
		// Unknown type: let lvcreate reject it (surfaced as a reconcile error).
		return 0, 0, true
	}
}

// resizeSlackPercent is the size delta (as a percentage of the observed size)
// tolerated before a resize is acted on. It absorbs extent rounding so a
// percentage-sized LV that already matches its target neither churns lvextend
// nor flags a spurious shrink on every reconcile tick.
const resizeSlackPercent = 1

// maybeResizeLV reconciles an existing LV towards its desired size. It grows
// the LV when the target exceeds the observed size, and returns a non-empty
// validation message when the target is smaller (an unsupported shrink) so the
// caller can surface an LVMValidationError. Shrinking is never performed:
// lvextend cannot shrink and doing so risks data loss.
//
// For absolute LVs the target is spec.SizeBytes. For percentage-sized LVs the
// target is derived from the current VG size (pct% of the VG; halved for raid1,
// which stores two copies), so a shrink is also detected when the VG was
// extended and the percentage subsequently lowered. The grow lvextend is issued
// with the original `-l <pct>%VG` so LVM does the exact extent math.
func (ctrl *LVMLogicalVolumeReconcileController) maybeResizeLV(
	ctx context.Context,
	logger *zap.Logger,
	spec *storage.LVMLogicalVolumeSpecSpec,
	vgSize uint64,
	observed uint64,
) (string, error) {
	// observed == 0 means the size could not be parsed yet; don't act on it.
	if observed == 0 {
		return "", nil
	}

	var (
		target uint64
		opts   lvm.LVExtendOptions
	)

	switch {
	case spec.SizePercentVG > 0:
		if vgSize == 0 {
			return "", nil
		}

		target = vgSize * uint64(spec.SizePercentVG) / 100
		if spec.Type == storage.LVMLogicalVolumeTypeRAID1 {
			target /= 2
		}

		opts = lvm.LVExtendOptions{SizePercentVG: spec.SizePercentVG}
	case spec.SizeBytes > 0:
		target = spec.SizeBytes
		opts = lvm.LVExtendOptions{SizeBytes: spec.SizeBytes}
	default:
		return "", nil
	}

	switch {
	case target*100 > observed*(100+resizeSlackPercent):
		// Grow.
		logger.Info(
			"growing LVM logical volume",
			zap.String("vg", spec.VGName),
			zap.String("lv", spec.Name),
			zap.Uint64("from_bytes", observed),
			zap.Uint64("target_bytes", target),
		)

		if err := ctrl.LVM.LVExtend(ctx, spec.VGName, spec.Name, opts); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				logger.Warn("lvm binary not found; skipping LVM provisioning")

				return "", nil
			}

			return "", fmt.Errorf("lvextend: %w", err)
		}

		return "", nil
	case target*100 < observed*(100-resizeSlackPercent):
		// Shrink requested: unsupported. Surface as a validation error.
		msg := fmt.Sprintf(
			"requested size %d bytes is smaller than current size %d bytes; shrinking logical volumes is not supported",
			target, observed,
		)

		logger.Warn(
			"refusing to shrink LVM logical volume",
			zap.String("vg", spec.VGName),
			zap.String("lv", spec.Name),
			zap.Uint64("from_bytes", observed),
			zap.Uint64("target_bytes", target),
		)

		return msg, nil
	default:
		// Within slack: nothing to do.
		return "", nil
	}
}
