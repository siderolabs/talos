// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// VolumeConfigController provides volume configuration based on Talos defaults and machine configuration.
type VolumeConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *VolumeConfigController) Name() string {
	return "block.VolumeConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VolumeConfigController) Inputs() []controller.Input {
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
func (ctrl *VolumeConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *VolumeConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

		// META volume discovery, always created unconditionally
		if err = safe.WriterModify(ctx, r,
			block.NewVolumeConfig(block.NamespaceName, constants.MetaPartitionLabel),
			func(vc *block.VolumeConfig) error {
				vc.TypedSpec().Type = block.VolumeTypePartition
				vc.TypedSpec().Locator = block.LocatorSpec{
					PartitionLabel: constants.MetaPartitionLabel,
				}

				return nil
			},
		); err != nil {
			return fmt.Errorf("error creating meta volume configuration: %w", err)
		}

		// if config is present
		// [TODO]: support custom configuration later
		if cfg != nil {
			if err = safe.WriterModify(ctx, r,
				block.NewVolumeConfig(block.NamespaceName, constants.StatePartitionLabel),
				func(vc *block.VolumeConfig) error {
					vc.TypedSpec().Type = block.VolumeTypePartition

					vc.TypedSpec().Provisioning = block.ProvisioningSpec{
						Wave: block.WaveSystemDisk,
						DiskSelector: block.DiskSelector{
							SystemDisk: true,
						},
						PartitionSpec: block.PartitionSpec{
							MinSize:  partition.StateSize,
							MaxSize:  partition.StateSize,
							Label:    constants.StatePartitionLabel,
							TypeUUID: partition.LinuxFilesystemData,
						},
					}

					vc.TypedSpec().Locator = block.LocatorSpec{
						PartitionLabel: constants.StatePartitionLabel,
					}

					return nil
				},
			); err != nil {
				return fmt.Errorf("error creating state volume configuration: %w", err)
			}

			if err = safe.WriterModify(ctx, r,
				block.NewVolumeConfig(block.NamespaceName, constants.EphemeralPartitionLabel),
				func(vc *block.VolumeConfig) error {
					vc.TypedSpec().Type = block.VolumeTypePartition

					vc.TypedSpec().Provisioning = block.ProvisioningSpec{
						Wave: block.WaveSystemDisk,
						DiskSelector: block.DiskSelector{
							SystemDisk: true,
						},
						PartitionSpec: block.PartitionSpec{
							MinSize:  partition.EphemeralMinSize,
							Grow:     true,
							Label:    constants.EphemeralPartitionLabel,
							TypeUUID: partition.LinuxFilesystemData,
						},
					}

					vc.TypedSpec().Locator = block.LocatorSpec{
						PartitionLabel: constants.EphemeralPartitionLabel,
					}

					return nil
				},
			); err != nil {
				return fmt.Errorf("error creating ephemeral volume configuration: %w", err)
			}
		}

		// [TODO]: this would fail as it doesn't handle finalizers properly
		if err = safe.CleanupOutputs[*block.VolumeConfig](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up volume configuration: %w", err)
		}
	}
}
