// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
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
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
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

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching machine configuration")
		}

		r.StartTrackingOutputs()

		configurationPresent := cfg != nil && cfg.Config().Machine() != nil

		if configurationPresent {
			// user disks
			for _, disk := range cfg.Config().Machine().Disks() {
				device := disk.Device()

				resolvedDevicePath, err := filepath.EvalSymlinks(device)
				if err != nil {
					return fmt.Errorf("error resolving device path: %w", err)
				}

				for idx, part := range disk.Partitions() {
					id := fmt.Sprintf("%s-%d", disk.Device(), idx+1)

					if err = safe.WriterModify(ctx, r,
						block.NewVolumeConfig(block.NamespaceName, id),
						func(vc *block.VolumeConfig) error {
							vc.Metadata().Labels().Set(block.UserDiskLabel, "")

							vc.TypedSpec().Type = block.VolumeTypePartition

							vc.TypedSpec().Provisioning = block.ProvisioningSpec{
								Wave: block.WaveUserDisks,
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

							// TODO: label user disks
							vc.TypedSpec().Mount = block.MountSpec{
								TargetPath: part.MountPoint(),
							}

							return nil
						},
					); err != nil {
						return fmt.Errorf("error creating user disk volume configuration: %w", err)
					}
				}
			}
		}

		if err = safe.CleanupOutputs[*block.VolumeConfig](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up volume configuration: %w", err)
		}

		if configurationPresent {
			if err = safe.WriterModify(ctx, r,
				block.NewUserDiskConfigStatus(block.NamespaceName, block.UserDiskConfigStatusID),
				func(udcs *block.UserDiskConfigStatus) error {
					udcs.TypedSpec().Ready = true

					return nil
				},
			); err != nil {
				return fmt.Errorf("error creating user disk configuration status: %w", err)
			}
		}
	}
}
