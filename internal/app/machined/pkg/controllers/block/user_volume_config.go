// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/partition"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// UserVolumeConfigController provides user volume configuration based on UserVolumeConfig, SwapVolumeConfig, etc. documents.
type UserVolumeConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *UserVolumeConfigController) Name() string {
	return "block.UserVolumeConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UserVolumeConfigController) Inputs() []controller.Input {
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
			Namespace: block.NamespaceName,
			Type:      block.VolumeConfigType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UserVolumeConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeConfigType,
			Kind: controller.OutputShared,
		},
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *UserVolumeConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

		// fetch all user-defined volume configs
		var (
			userVolumeConfigs []configconfig.UserVolumeConfig
			swapVolumeConfigs []configconfig.SwapVolumeConfig
		)

		if cfg != nil {
			userVolumeConfigs = cfg.Config().UserVolumeConfigs()
			swapVolumeConfigs = cfg.Config().SwapVolumeConfigs()
		}

		// list of all labels for VolumeConfig and VolumeMountRequest resources that are managed by this controller
		labelQuery := []state.ListOption{
			state.WithLabelQuery(resource.LabelExists(block.UserVolumeLabel)),
			state.WithLabelQuery(resource.LabelExists(block.SwapVolumeLabel)),
		}

		volumeConfigs, err := safe.ReaderListAll[*block.VolumeConfig](ctx, r, labelQuery...)
		if err != nil {
			return fmt.Errorf("error fetching volume configs: %w", err)
		}

		volumeConfigsByID := xslices.ToMap(
			safe.ToSlice(volumeConfigs, identity),
			func(v *block.VolumeConfig) (resource.ID, *block.VolumeConfig) {
				return v.Metadata().ID(), v
			},
		)

		volumeMountRequests, err := safe.ReaderListAll[*block.VolumeMountRequest](ctx, r, labelQuery...)
		if err != nil {
			return fmt.Errorf("error fetching volume mount requests: %w", err)
		}

		volumeMountRequestsByID := xslices.ToMap(
			safe.ToSlice(volumeMountRequests, identity),
			func(v *block.VolumeMountRequest) (resource.ID, *block.VolumeMountRequest) {
				return v.Metadata().ID(), v
			},
		)

		for _, userVolumeConfig := range userVolumeConfigs {
			if err := handleCustomVolumeConfig(
				ctx, r, constants.UserVolumePrefix, block.UserVolumeLabel, ctrl.Name(),
				userVolumeConfig, volumeConfigsByID, volumeMountRequestsByID,
				ctrl.handleUserVolumeConfig,
			); err != nil {
				return fmt.Errorf("error handling user volume config %q: %w", userVolumeConfig.Name(), err)
			}
		}

		for _, swapVolumeConfig := range swapVolumeConfigs {
			if err := handleCustomVolumeConfig(
				ctx, r, constants.SwapVolumePrefix, block.SwapVolumeLabel, ctrl.Name(),
				swapVolumeConfig, volumeConfigsByID, volumeMountRequestsByID,
				ctrl.handleSwapVolumeConfig,
			); err != nil {
				return fmt.Errorf("error handling swap volume config %q: %w", swapVolumeConfig.Name(), err)
			}
		}

		// whatever is left in the maps should be torn down & destroyed
		for _, volumeConfig := range volumeConfigsByID {
			okToDestroy, err := r.Teardown(ctx, volumeConfig.Metadata())
			if err != nil {
				return fmt.Errorf("error tearing down volume config %q: %w", volumeConfig.Metadata().ID(), err)
			}

			if okToDestroy {
				if err = r.Destroy(ctx, volumeConfig.Metadata()); err != nil {
					return fmt.Errorf("error destroying volume config %q: %w", volumeConfig.Metadata().ID(), err)
				}
			}
		}

		for _, volumeMountRequest := range volumeMountRequestsByID {
			okToDestroy, err := r.Teardown(ctx, volumeMountRequest.Metadata())
			if err != nil {
				return fmt.Errorf("error tearing down volume mount request %q: %w", volumeMountRequest.Metadata().ID(), err)
			}

			if okToDestroy {
				if err = r.Destroy(ctx, volumeMountRequest.Metadata()); err != nil {
					return fmt.Errorf("error destroying volume mount request %q: %w", volumeMountRequest.Metadata().ID(), err)
				}
			}
		}
	}
}

// handleCustomVolumeConfig handled transormation of a custom (user) volume configuration
// into VolumeConfig and VolumeMountRequest resources.
//
// The function is generic accepting some common properties:
// - prefix is used to create the volume ID from the config document name
// - label is used to set the label on the VolumeConfig/VolumeMountRequest
// - transformFunc is a function that transforms the config document into VolumeConfig spec.
func handleCustomVolumeConfig[C configconfig.NamedDocument](
	ctx context.Context, r controller.ReaderWriter,
	prefix, label, requester string,
	configDocument C,
	volumeConfigsByID map[string]*block.VolumeConfig,
	volumeMountRequestsByID map[string]*block.VolumeMountRequest,
	transformFunc func(c C, v *block.VolumeConfig, volumeID string) error,
) error {
	volumeID := prefix + configDocument.Name()

	volumeConfig := volumeConfigsByID[volumeID]
	volumeMountRequest := volumeMountRequestsByID[volumeID]

	tearingDown := (volumeConfig != nil && volumeConfig.Metadata().Phase() == resource.PhaseTearingDown) ||
		(volumeMountRequest != nil && volumeMountRequest.Metadata().Phase() == resource.PhaseTearingDown)

		// if the volume is being torn down, do the tear down (in the next loop)
	if tearingDown {
		return nil
	}

	delete(volumeConfigsByID, volumeID)
	delete(volumeMountRequestsByID, volumeID)

	if err := safe.WriterModify(ctx, r,
		block.NewVolumeConfig(block.NamespaceName, volumeID),
		func(v *block.VolumeConfig) error {
			v.Metadata().Labels().Set(label, "")

			return transformFunc(configDocument, v, volumeID)
		},
	); err != nil {
		return fmt.Errorf("error creating volume configuration: %w", err)
	}

	if err := safe.WriterModify(ctx, r,
		block.NewVolumeMountRequest(block.NamespaceName, volumeID),
		func(v *block.VolumeMountRequest) error {
			v.Metadata().Labels().Set(block.UserVolumeLabel, "")
			v.TypedSpec().Requester = requester
			v.TypedSpec().VolumeID = volumeID

			return nil
		},
	); err != nil {
		return fmt.Errorf("error creating volume mount request: %w", err)
	}

	return nil
}

func (ctrl *UserVolumeConfigController) handleUserVolumeConfig(
	userVolumeConfig configconfig.UserVolumeConfig,
	v *block.VolumeConfig,
	volumeID string,
) error {
	diskSelector, ok := userVolumeConfig.Provisioning().DiskSelector().Get()
	if !ok {
		// this shouldn't happen due to validation
		return fmt.Errorf("disk selector not found for volume %q", volumeID)
	}

	v.TypedSpec().Type = block.VolumeTypePartition
	v.TypedSpec().Locator.Match = labelVolumeMatch(volumeID)
	v.TypedSpec().Provisioning = block.ProvisioningSpec{
		Wave: block.WaveUserVolumes,
		DiskSelector: block.DiskSelector{
			Match: diskSelector,
		},
		PartitionSpec: block.PartitionSpec{
			MinSize:  userVolumeConfig.Provisioning().MinSize().ValueOrZero(),
			MaxSize:  userVolumeConfig.Provisioning().MaxSize().ValueOrZero(),
			Grow:     userVolumeConfig.Provisioning().Grow().ValueOrZero(),
			Label:    volumeID,
			TypeUUID: partition.LinuxFilesystemData,
		},
		FilesystemSpec: block.FilesystemSpec{
			Type: userVolumeConfig.Filesystem().Type(),
		},
	}
	v.TypedSpec().Mount = block.MountSpec{
		TargetPath:   userVolumeConfig.Name(),
		ParentID:     constants.UserVolumeMountPoint,
		SelinuxLabel: constants.EphemeralSelinuxLabel,
		FileMode:     0o755,
		UID:          0,
		GID:          0,
	}

	if err := convertEncryptionConfiguration(userVolumeConfig.Encryption(), v.TypedSpec()); err != nil {
		return fmt.Errorf("error apply encryption configuration: %w", err)
	}

	return nil
}

func (ctrl *UserVolumeConfigController) handleSwapVolumeConfig(
	swapVolumeConfig configconfig.SwapVolumeConfig,
	v *block.VolumeConfig,
	volumeID string,
) error {
	diskSelector, ok := swapVolumeConfig.Provisioning().DiskSelector().Get()
	if !ok {
		// this shouldn't happen due to validation
		return fmt.Errorf("disk selector not found for volume %q", volumeID)
	}

	v.TypedSpec().Type = block.VolumeTypePartition
	v.TypedSpec().Locator.Match = labelVolumeMatch(volumeID)
	v.TypedSpec().Provisioning = block.ProvisioningSpec{
		Wave: block.WaveUserVolumes,
		DiskSelector: block.DiskSelector{
			Match: diskSelector,
		},
		PartitionSpec: block.PartitionSpec{
			MinSize:  swapVolumeConfig.Provisioning().MinSize().ValueOrZero(),
			MaxSize:  swapVolumeConfig.Provisioning().MaxSize().ValueOrZero(),
			Grow:     swapVolumeConfig.Provisioning().Grow().ValueOrZero(),
			Label:    volumeID,
			TypeUUID: partition.LinkSwap,
		},
		FilesystemSpec: block.FilesystemSpec{
			Type: block.FilesystemTypeSwap,
		},
	}

	if err := convertEncryptionConfiguration(swapVolumeConfig.Encryption(), v.TypedSpec()); err != nil {
		return fmt.Errorf("error apply encryption configuration: %w", err)
	}

	return nil
}
