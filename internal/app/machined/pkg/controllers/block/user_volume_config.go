// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"cmp"
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/partition"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// Size constants.
const (
	MiB               = 1024 * 1024
	MinUserVolumeSize = 100 * MiB
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

		// create a volume mount request for the root user volume mount point
		// to keep it alive and prevent it from being torn down
		if err := safe.WriterModify(ctx, r,
			block.NewVolumeMountRequest(block.NamespaceName, constants.UserVolumeMountPoint),
			func(v *block.VolumeMountRequest) error {
				v.TypedSpec().Requester = ctrl.Name()
				v.TypedSpec().VolumeID = constants.UserVolumeMountPoint

				return nil
			},
		); err != nil {
			return fmt.Errorf("error creating volume mount request for user volume mount point: %w", err)
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching machine configuration")
		}

		// fetch all user-defined volume configs
		var (
			userVolumeConfigs     []configconfig.UserVolumeConfig
			rawVolumeConfigs      []configconfig.RawVolumeConfig
			existingVolumeConfigs []configconfig.ExistingVolumeConfig
			swapVolumeConfigs     []configconfig.SwapVolumeConfig
		)

		if cfg != nil {
			userVolumeConfigs = cfg.Config().UserVolumeConfigs()
			rawVolumeConfigs = cfg.Config().RawVolumeConfigs()
			existingVolumeConfigs = cfg.Config().ExistingVolumeConfigs()
			swapVolumeConfigs = cfg.Config().SwapVolumeConfigs()
		}

		// list of all labels for VolumeConfig and VolumeMountRequest resources that are managed by this controller
		labelQuery := []state.ListOption{
			state.WithLabelQuery(resource.LabelExists(block.UserVolumeLabel)),
			state.WithLabelQuery(resource.LabelExists(block.RawVolumeLabel)),
			state.WithLabelQuery(resource.LabelExists(block.ExistingVolumeLabel)),
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
				defaultMountTransform,
			); err != nil {
				return fmt.Errorf("error handling user volume config %q: %w", userVolumeConfig.Name(), err)
			}
		}

		for _, rawVolumeConfig := range rawVolumeConfigs {
			if err := handleCustomVolumeConfig(
				ctx, r, constants.RawVolumePrefix, block.RawVolumeLabel, ctrl.Name(),
				rawVolumeConfig, volumeConfigsByID, volumeMountRequestsByID,
				ctrl.handleRawVolumeConfig,
				skipMountTransform,
			); err != nil {
				return fmt.Errorf("error handling raw volume config %q: %w", rawVolumeConfig.Name(), err)
			}
		}

		for _, existingVolumeConfig := range existingVolumeConfigs {
			if err := handleCustomVolumeConfig(
				ctx, r, constants.ExistingVolumePrefix, block.ExistingVolumeLabel, ctrl.Name(),
				existingVolumeConfig, volumeConfigsByID, volumeMountRequestsByID,
				ctrl.handleExistingVolumeConfig,
				ctrl.handleExistinVolumeMountRequest,
			); err != nil {
				return fmt.Errorf("error handling existing volume config %q: %w", existingVolumeConfig.Name(), err)
			}
		}

		for _, swapVolumeConfig := range swapVolumeConfigs {
			if err := handleCustomVolumeConfig(
				ctx, r, constants.SwapVolumePrefix, block.SwapVolumeLabel, ctrl.Name(),
				swapVolumeConfig, volumeConfigsByID, volumeMountRequestsByID,
				ctrl.handleSwapVolumeConfig,
				defaultMountTransform,
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

// skipUserVolumeMountRequest is used to skip creating a VolumeMountRequest for a user volume.
type skipUserVolumeMountRequest struct{}

func defaultMountTransform[C configconfig.NamedDocument](C, *block.VolumeMountRequest, string) error {
	return nil
}

func skipMountTransform[C configconfig.NamedDocument](C, *block.VolumeMountRequest, string) error {
	return xerrors.NewTaggedf[skipUserVolumeMountRequest]("skip")
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
	mountTransformFunc func(c C, m *block.VolumeMountRequest, volumeID string) error,
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

			return mountTransformFunc(configDocument, v, volumeID)
		},
	); err != nil {
		if !xerrors.TagIs[skipUserVolumeMountRequest](err) {
			return fmt.Errorf("error creating volume mount request: %w", err)
		}
	}

	return nil
}

func (ctrl *UserVolumeConfigController) handleUserVolumeConfig(
	userVolumeConfig configconfig.UserVolumeConfig,
	v *block.VolumeConfig,
	volumeID string,
) error {
	switch userVolumeConfig.Type().ValueOr(block.VolumeTypePartition) {
	case block.VolumeTypePartition:
		return ctrl.handlePartitionUserVolumeConfig(userVolumeConfig, v, volumeID)

	case block.VolumeTypeDirectory:
		return ctrl.handleDirectoryUserVolumeConfig(userVolumeConfig, v)

	case block.VolumeTypeDisk, block.VolumeTypeTmpfs, block.VolumeTypeSymlink, block.VolumeTypeOverlay:
		fallthrough

	default:
		return fmt.Errorf("unsupported volume type %q", userVolumeConfig.Type().ValueOr(block.VolumeTypePartition).String())
	}
}

func (ctrl *UserVolumeConfigController) handlePartitionUserVolumeConfig(
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
			MinSize:  cmp.Or(userVolumeConfig.Provisioning().MinSize().ValueOrZero(), MinUserVolumeSize),
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
		TargetPath:          userVolumeConfig.Name(),
		ParentID:            constants.UserVolumeMountPoint,
		SelinuxLabel:        constants.EphemeralSelinuxLabel,
		FileMode:            0o755,
		UID:                 0,
		GID:                 0,
		ProjectQuotaSupport: userVolumeConfig.Filesystem().ProjectQuotaSupport(),
	}

	if err := convertEncryptionConfiguration(userVolumeConfig.Encryption(), v.TypedSpec()); err != nil {
		return fmt.Errorf("error apply encryption configuration: %w", err)
	}

	return nil
}

func (ctrl *UserVolumeConfigController) handleDirectoryUserVolumeConfig(
	userVolumeConfig configconfig.UserVolumeConfig,
	v *block.VolumeConfig,
) error {
	v.TypedSpec().Type = block.VolumeTypeDirectory
	v.TypedSpec().Mount = block.MountSpec{
		TargetPath:   userVolumeConfig.Name(),
		ParentID:     constants.UserVolumeMountPoint,
		SelinuxLabel: constants.EphemeralSelinuxLabel,
		FileMode:     0o755,
		UID:          0,
		GID:          0,
		BindTarget:   pointer.To(userVolumeConfig.Name()),
	}

	return nil
}

//nolint:dupl
func (ctrl *UserVolumeConfigController) handleRawVolumeConfig(
	rawVolumeConfig configconfig.RawVolumeConfig,
	v *block.VolumeConfig,
	volumeID string,
) error {
	diskSelector, ok := rawVolumeConfig.Provisioning().DiskSelector().Get()
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
			MinSize:  cmp.Or(rawVolumeConfig.Provisioning().MinSize().ValueOrZero(), MinUserVolumeSize),
			MaxSize:  rawVolumeConfig.Provisioning().MaxSize().ValueOrZero(),
			Grow:     rawVolumeConfig.Provisioning().Grow().ValueOrZero(),
			Label:    volumeID,
			TypeUUID: partition.LinuxFilesystemData,
		},
		FilesystemSpec: block.FilesystemSpec{
			Type: block.FilesystemTypeNone,
		},
	}

	if err := convertEncryptionConfiguration(rawVolumeConfig.Encryption(), v.TypedSpec()); err != nil {
		return fmt.Errorf("error apply encryption configuration: %w", err)
	}

	return nil
}

func (ctrl *UserVolumeConfigController) handleExistingVolumeConfig(
	existingVolumeConfig configconfig.ExistingVolumeConfig,
	v *block.VolumeConfig,
	volumeID string,
) error {
	v.TypedSpec().Type = block.VolumeTypePartition
	v.TypedSpec().Locator.Match = existingVolumeConfig.VolumeDiscovery().VolumeSelector()
	v.TypedSpec().Mount = block.MountSpec{
		TargetPath:   existingVolumeConfig.Name(),
		ParentID:     constants.UserVolumeMountPoint,
		SelinuxLabel: constants.EphemeralSelinuxLabel,
		FileMode:     0o755,
		UID:          0,
		GID:          0,
	}

	return nil
}

func (ctrl *UserVolumeConfigController) handleExistinVolumeMountRequest(
	existingVolumeConfig configconfig.ExistingVolumeConfig,
	m *block.VolumeMountRequest,
	_ string,
) error {
	m.TypedSpec().ReadOnly = existingVolumeConfig.Mount().ReadOnly()

	return nil
}

//nolint:dupl
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
			MaxSize:  cmp.Or(swapVolumeConfig.Provisioning().MaxSize().ValueOrZero(), MinUserVolumeSize),
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
