// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	machineconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// UserDiskConfigController provides volume configuration based on Talos v1alpha1 user disks.
type UserDiskConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *UserDiskConfigController) Name() string {
	return "block.UserDiskConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UserDiskConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: resources.InMemoryNamespace,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UserDiskConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeConfigType,
			Kind: controller.OutputShared,
		},
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
		{
			Type: block.UserDiskConfigStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

func diskPathMatch(devicePath string) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("disk.dev_path == '%s'", devicePath), celenv.DiskLocator()))
}

func partitionIdxMatch(devicePath string, partitionIdx int) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.parent_dev_path == '%s' && volume.partition_index == %du", devicePath, partitionIdx), celenv.VolumeLocator()))
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *UserDiskConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching machine configuration")
		}

		r.StartTrackingOutputs()

		configurationPresent := cfg != nil && cfg.Config().Machine() != nil

		status := block.NewUserDiskConfigStatus(block.NamespaceName, block.UserDiskConfigStatusID)
		status.TypedSpec().Ready = configurationPresent
		status.TypedSpec().TornDown = true

		if configurationPresent {
			// user disks
			for _, disk := range cfg.Config().Machine().Disks() {
				result, err := ctrl.processUserDisk(ctx, r, disk)
				if err != nil {
					return fmt.Errorf("error processing user disk %s: %w", disk.Device(), err)
				}

				status.TypedSpec().Ready = status.TypedSpec().Ready && result.ready
				status.TypedSpec().TornDown = status.TypedSpec().TornDown && result.tornDown
			}
		}

		if err = safe.CleanupOutputs[*block.VolumeConfig](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up volume configuration: %w", err)
		}

		if configurationPresent {
			if err = safe.WriterModify(ctx, r,
				block.NewUserDiskConfigStatus(block.NamespaceName, block.UserDiskConfigStatusID),
				func(udcs *block.UserDiskConfigStatus) error {
					*udcs.TypedSpec() = *status.TypedSpec()

					return nil
				},
			); err != nil {
				return fmt.Errorf("error creating user disk configuration status: %w", err)
			}
		}
	}
}

type userDiskResult struct {
	ready    bool
	tornDown bool
}

func (ctrl *UserDiskConfigController) processUserDisk(ctx context.Context, r controller.ReaderWriter, disk machineconfig.Disk) (*userDiskResult, error) {
	device := disk.Device()

	resolvedDevicePath, err := filepath.EvalSymlinks(device)
	if err != nil {
		return nil, fmt.Errorf("error resolving device path: %w", err)
	}

	overallResult := &userDiskResult{
		ready:    true,
		tornDown: true,
	}

	for idx, part := range disk.Partitions() {
		result, err := ctrl.processUserDiskPartition(ctx, r, device, idx, part, resolvedDevicePath)
		if err != nil {
			return nil, fmt.Errorf("error processing user disk partition %s: %w", part.MountPoint(), err)
		}

		overallResult.ready = overallResult.ready && result.ready
		overallResult.tornDown = overallResult.tornDown && result.tornDown
	}

	return overallResult, nil
}

//nolint:gocyclo,cyclop
func (ctrl *UserDiskConfigController) processUserDiskPartition(
	ctx context.Context, r controller.ReaderWriter, device string, idx int, part machineconfig.Partition, resolvedDevicePath string,
) (*userDiskResult, error) {
	id := fmt.Sprintf("%s-%d", device, idx+1)

	// volume configuration
	if err := safe.WriterModify(ctx, r,
		block.NewVolumeConfig(block.NamespaceName, id),
		func(vc *block.VolumeConfig) error {
			vc.Metadata().Labels().Set(block.UserDiskLabel, "")

			vc.TypedSpec().Type = block.VolumeTypePartition

			vc.TypedSpec().Provisioning = block.ProvisioningSpec{
				// it's crucial to keep the order of provisioning locked within each disk, otherwise
				// provisioning might order them different way, and create partitions in wrong order
				// the matcher on partition idx would then discover partitions in wrong order, and mount them
				// in wrong order
				Wave: block.WaveLegacyUserDisks + idx,
				DiskSelector: block.DiskSelector{
					Match: diskPathMatch(resolvedDevicePath),
				},
				PartitionSpec: block.PartitionSpec{
					MinSize:  part.Size(),
					MaxSize:  part.Size(),
					TypeUUID: partition.LinuxFilesystemData,
				},
				FilesystemSpec: block.FilesystemSpec{
					Type: block.FilesystemTypeXFS,
				},
			}

			vc.TypedSpec().Locator = block.LocatorSpec{
				Match: partitionIdxMatch(resolvedDevicePath, idx+1),
			}

			targetPath := part.MountPoint()
			parentID := ""

			// machine configuration doesn't enforce any specific mount point for user disks,
			// so we don't do any more thorough validation here
			if strings.HasPrefix(targetPath, "/var/") {
				parentID = constants.EphemeralPartitionLabel
				targetPath = strings.TrimPrefix(targetPath, "/var/")
			}

			vc.TypedSpec().Mount = block.MountSpec{
				TargetPath:   targetPath,
				ParentID:     parentID,
				SelinuxLabel: constants.EphemeralSelinuxLabel,
				FileMode:     0o755,
				UID:          0,
				GID:          0,
			}

			return nil
		},
	); err != nil {
		return nil, fmt.Errorf("error creating user disk volume configuration: %w", err)
	}

	// figure out if we want to create the mount of to tear it down
	volumeMountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error fetching volume mount status: %w", err)
	}

	volumeMountRequest, err := safe.ReaderGetByID[*block.VolumeMountRequest](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error fetching volume mount request: %w", err)
	}

	shouldTearDown := (volumeMountStatus != nil && volumeMountStatus.Metadata().Phase() == resource.PhaseTearingDown) ||
		(volumeMountRequest != nil && volumeMountRequest.Metadata().Phase() == resource.PhaseTearingDown)

	if !shouldTearDown {
		// create volume mount request
		if err = safe.WriterModify(ctx, r,
			block.NewVolumeMountRequest(block.NamespaceName, id),
			func(vmr *block.VolumeMountRequest) error {
				vmr.TypedSpec().Requester = ctrl.Name()
				vmr.TypedSpec().VolumeID = id

				return nil
			},
		); err != nil {
			return nil, fmt.Errorf("error creating volume mount request: %w", err)
		}

		if volumeMountStatus == nil {
			// not mounted yet
			return &userDiskResult{}, nil
		}

		if !volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			if err = r.AddFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
				return nil, fmt.Errorf("error adding finalizer to volume mount status: %w", err)
			}
		}

		// ready
		return &userDiskResult{
			ready: true,
		}, nil
	}

	// tear down volume mount request
	if volumeMountStatus != nil && volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = r.RemoveFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
			return nil, fmt.Errorf("error removing finalizer from volume mount status: %w", err)
		}
	}

	if volumeMountRequest == nil {
		// already torn down
		return &userDiskResult{
			tornDown: true,
		}, nil
	}

	okToDestroy, err := r.Teardown(ctx, volumeMountRequest.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return &userDiskResult{
				tornDown: true,
			}, nil
		}

		return nil, fmt.Errorf("error tearing down volume mount request: %w", err)
	}

	if !okToDestroy {
		return &userDiskResult{}, nil
	}

	if err = r.Destroy(ctx, volumeMountRequest.Metadata()); err != nil {
		return nil, fmt.Errorf("error destroying volume mount request: %w", err)
	}

	return &userDiskResult{
		tornDown: true,
	}, nil
}
