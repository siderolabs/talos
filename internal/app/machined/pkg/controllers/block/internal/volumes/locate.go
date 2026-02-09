// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/partitioning"
	"go.uber.org/zap"

	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// LocateAndProvision locates and provisions a volume.
func LocateAndProvision(ctx context.Context, logger *zap.Logger, vc ManagerContext) error {
	// 1. Setup common status fields
	vc.Status.MountSpec = vc.Cfg.TypedSpec().Mount
	vc.Status.SymlinkSpec = vc.Cfg.TypedSpec().Symlink

	// 2. Handle simple types (Tmpfs, Overlay, External, etc.)
	// If handled, we return early.
	if done := handleSimpleVolumeTypes(vc); done {
		return nil
	}

	// 3. Validation for Disk/Partition types
	if value.IsZero(vc.Cfg.TypedSpec().Locator) {
		return fmt.Errorf("volume locator is not set")
	}

	// 4. Attempt to locate an existing volume
	located, err := locateExistingVolume(vc)
	if err != nil {
		return err
	}

	if located {
		return nil
	}

	// 5. Handle Waiting State
	// If not found and devices aren't ready, we must wait.
	if !vc.DevicesReady {
		vc.Status.Phase = block.VolumePhaseWaiting

		return nil
	}

	// 6. Provision new volume
	return provisionNewVolume(ctx, logger, vc)
}

// handleSimpleVolumeTypes handles non-provisionable types.
// Returns true if the volume type was handled.
func handleSimpleVolumeTypes(vc ManagerContext) bool {
	spec := vc.Cfg.TypedSpec()

	switch spec.Type {
	case block.VolumeTypeTmpfs, block.VolumeTypeDirectory, block.VolumeTypeSymlink, block.VolumeTypeOverlay:
		vc.Status.Phase = block.VolumePhaseReady

		return true

	case block.VolumeTypeExternal:
		vc.Status.Phase = block.VolumePhaseReady
		vc.Status.Filesystem = spec.Provisioning.FilesystemSpec.Type
		vc.Status.Location = spec.Provisioning.DiskSelector.External
		vc.Status.MountLocation = spec.Provisioning.DiskSelector.External

		return true

	case block.VolumeTypeDisk, block.VolumeTypePartition:
		fallthrough

	default:
		return false
	}
}

// locateExistingVolume iterates through discovered volumes or disks to find a match.
//
// For disk volumes with a DiskMatch locator the function iterates over disks
// (each disk is evaluated exactly once) and then picks the best discovered
// volume for the matched disk.  For all other volume types it iterates over
// discovered volumes and returns on the first match.
func locateExistingVolume(vc ManagerContext) (bool, error) {
	spec := vc.Cfg.TypedSpec()

	switch {
	case !spec.Locator.DiskMatch.IsZero():
		if spec.Type != block.VolumeTypeDisk {
			return false, fmt.Errorf("DiskMatch locator is only valid for disk volumes")
		}

		return locateDiskByDiskMatch(vc)
	case !spec.Locator.Match.IsZero():
		return locateVolumeByMatch(vc)
	default:
		return false, fmt.Errorf("no locator expression set for volume")
	}
}

// locateDiskByDiskMatch handles VolumeTypeDisk with Locator.DiskMatch.
//
// It iterates over disks (not discovered volumes), so each physical disk is
// evaluated exactly once. If more than one disk matches, an error is returned.
// The best discovered volume for the matched disk is then selected, preferring
// whole-disk entries over partition entries.
//
//nolint:gocyclo
func locateDiskByDiskMatch(vc ManagerContext) (bool, error) {
	spec := vc.Cfg.TypedSpec()
	env := celenv.DiskLocator()

	var matchedDisks []string

	for _, diskCtx := range vc.Disks {
		matches, err := spec.Locator.DiskMatch.EvalBool(env, map[string]any{"disk": diskCtx.Disk})
		if err != nil {
			return false, fmt.Errorf("error evaluating disk locator: %w", err)
		}

		if matches {
			matchedDisks = append(matchedDisks, diskCtx.Disk.DevPath)
		}
	}

	if len(matchedDisks) > 1 {
		return false, fmt.Errorf("multiple disks matched locator for disk volume; matched disks: %v", matchedDisks)
	}

	if len(matchedDisks) == 0 {
		return false, nil
	}

	diskDev := matchedDisks[0]

	// Find the best discovered volume for this disk, preferring whole-disk
	// entries (ParentDevPath == "") over partition entries.
	var matchedVol *blockpb.DiscoveredVolumeSpec

	for _, dv := range vc.DiscoveredVolumes {
		if dv.DevPath != diskDev && dv.ParentDevPath != diskDev {
			continue
		}

		if matchedVol == nil || (matchedVol.ParentDevPath != "" && dv.ParentDevPath == "") {
			matchedVol = dv
		}
	}

	if matchedVol != nil {
		applyLocatedStatus(vc, matchedVol)

		return true, nil
	}

	return false, nil
}

// locateVolumeByMatch handles volumes with Locator.Match (a CEL expression
// evaluated against each discovered volume).
func locateVolumeByMatch(vc ManagerContext) (bool, error) {
	spec := vc.Cfg.TypedSpec()
	env := celenv.VolumeLocator()

	for _, dv := range vc.DiscoveredVolumes {
		matchContext := map[string]any{"volume": dv}

		// Resolve the parent disk for CEL context
		for _, diskCtx := range vc.Disks {
			if (dv.ParentDevPath != "" && diskCtx.Disk.DevPath == dv.ParentDevPath) ||
				(dv.ParentDevPath == "" && diskCtx.Disk.DevPath == dv.DevPath) {
				matchContext["disk"] = diskCtx.Disk

				break
			}
		}

		matches, err := spec.Locator.Match.EvalBool(env, matchContext)
		if err != nil {
			return false, fmt.Errorf("error evaluating volume locator: %w", err)
		}

		if matches {
			applyLocatedStatus(vc, dv)

			return true, nil
		}
	}

	return false, nil
}

// applyLocatedStatus updates the status when a volume is found.
func applyLocatedStatus(vc ManagerContext, vol *blockpb.DiscoveredVolumeSpec) {
	vc.Status.Phase = block.VolumePhaseLocated
	vc.Status.Location = vol.DevPath
	vc.Status.PartitionIndex = int(vol.PartitionIndex)
	vc.Status.ParentLocation = vol.ParentDevPath
	vc.Status.UUID = vol.Uuid
	vc.Status.PartitionUUID = vol.PartitionUuid
	vc.Status.SetSize(vol.Size)
}

// provisionNewVolume handles the creation/provisioning of missing volumes.
func provisionNewVolume(ctx context.Context, logger *zap.Logger, vc ManagerContext) error {
	spec := vc.Cfg.TypedSpec()

	// Pre-checks
	if value.IsZero(spec.Provisioning) {
		vc.Status.Phase = block.VolumePhaseMissing

		return nil
	}

	if !vc.PreviousWaveProvisioned {
		vc.Status.Phase = block.VolumePhaseWaiting

		return nil
	}

	// 1. Find candidate disks
	candidates, err := findCandidateDisks(vc)
	if err != nil {
		return err
	}

	logger.Debug("matched disks", zap.Strings("disks", candidates))

	// 2. Select the best fit
	pickedDisk, diskRes, err := selectBestDisk(logger, candidates, vc.Cfg)
	if err != nil {
		return err
	}

	logger.Debug("picked disk", zap.String("disk", pickedDisk))

	// 3. Apply Provisioning (Update status or Create Partition)
	return applyProvisioning(ctx, logger, vc, pickedDisk, diskRes)
}

// findCandidateDisks filters available disks based on the selector.
func findCandidateDisks(vc ManagerContext) ([]string, error) {
	var matchedDisks []string

	spec := vc.Cfg.TypedSpec()

	for _, diskCtx := range vc.Disks {
		if diskCtx.Disk.Readonly {
			continue
		}

		matches, err := spec.Provisioning.DiskSelector.Match.EvalBool(celenv.DiskLocator(), diskCtx.ToCELContext())
		if err != nil {
			return nil, fmt.Errorf("error evaluating disk locator: %w", err)
		}

		if matches {
			matchedDisks = append(matchedDisks, diskCtx.Disk.DevPath)
		}
	}

	if len(matchedDisks) == 0 {
		return nil, fmt.Errorf("no disks matched selector for volume")
	}

	if spec.Type == block.VolumeTypeDisk && len(matchedDisks) > 1 {
		return nil, fmt.Errorf("multiple disks matched locator for disk volume; matched disks: %v", matchedDisks)
	}

	return matchedDisks, nil
}

// selectBestDisk analyzes candidates and picks the one that satisfies constraints.
func selectBestDisk(logger *zap.Logger, candidates []string, cfg *block.VolumeConfig) (string, CheckDiskResult, error) {
	var (
		pickedDisk      string
		finalResult     CheckDiskResult
		rejectedReasons = map[DiskRejectedReason]int{}
	)

	for _, disk := range candidates {
		res := CheckDiskForProvisioning(logger, disk, cfg)
		if res.CanProvision {
			pickedDisk = disk
			finalResult = res

			break
		}

		rejectedReasons[res.RejectedReason]++
	}

	if pickedDisk == "" {
		err := xerrors.NewTaggedf[Retryable](
			"no disks matched for volume (%d matched selector): %d have not enough space, %d have wrong format, %d have other issues",
			len(candidates),
			rejectedReasons[NotEnoughSpace],
			rejectedReasons[WrongFormat],
			rejectedReasons[GeneralError],
		)

		return "", CheckDiskResult{}, err
	}

	return pickedDisk, finalResult, nil
}

// applyProvisioning performs the final provisioning step.
func applyProvisioning(ctx context.Context, logger *zap.Logger, vc ManagerContext, disk string, res CheckDiskResult) error {
	switch vc.Cfg.TypedSpec().Type {
	case block.VolumeTypeDisk:
		vc.Status.Phase = block.VolumePhaseProvisioned
		vc.Status.Location = disk
		vc.Status.ParentLocation = ""
		vc.Status.SetSize(res.DiskSize)

	case block.VolumeTypePartition:
		partRes, err := CreatePartition(ctx, logger, disk, vc.Cfg, res.HasGPT)
		if err != nil {
			return fmt.Errorf("error creating partition: %w", err)
		}

		vc.Status.Phase = block.VolumePhaseProvisioned
		vc.Status.Location = partitioning.DevName(disk, uint(partRes.PartitionIdx))
		vc.Status.PartitionIndex = partRes.PartitionIdx
		vc.Status.ParentLocation = disk
		vc.Status.PartitionUUID = partRes.Partition.PartGUID.String()
		vc.Status.SetSize(partRes.Size)

	case block.VolumeTypeTmpfs, block.VolumeTypeDirectory, block.VolumeTypeSymlink, block.VolumeTypeOverlay, block.VolumeTypeExternal:
		fallthrough

	default:
		panic(fmt.Sprintf("unexpected volume type: %s", vc.Cfg.TypedSpec().Type))
	}

	return nil
}
