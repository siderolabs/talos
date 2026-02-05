// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"cmp"
	"errors"
	"fmt"

	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/internal/pkg/partition"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Size constants.
const (
	MiB               = 1024 * 1024
	MinUserVolumeSize = 100 * MiB
)

// UserVolumeTransformers contains all the user volume config transformers.
var UserVolumeTransformers = []volumeConfigTransformer{
	UserVolumeTransformer,
	RawVolumeTransformer,
	ExistingVolumeTransformer,
	ExternalVolumeTransformer,
	SwapVolumeTransformer,
}

// UserVolumeTransformer is the transformer for user volume configs.
func UserVolumeTransformer(c configconfig.Config) ([]VolumeResource, error) {
	if c == nil {
		return nil, nil
	}

	resources := make([]VolumeResource, 0, len(c.UserVolumeConfigs()))

	for _, userVolumeConfig := range c.UserVolumeConfigs() {
		volumeID := constants.UserVolumePrefix + userVolumeConfig.Name()
		userVolumeResource := VolumeResource{
			VolumeID:           volumeID,
			Label:              block.UserVolumeLabel,
			MountTransformFunc: HandleUserVolumeMountRequest(userVolumeConfig), // This is overridden for Directory type below.
		}

		switch userVolumeConfig.Type().ValueOr(block.VolumeTypePartition) {
		case block.VolumeTypeDirectory:
			userVolumeResource.MountTransformFunc = DefaultMountTransform
			userVolumeResource.TransformFunc = NewBuilder().
				WithType(block.VolumeTypeDirectory).
				WithMount(block.MountSpec{
					TargetPath:   userVolumeConfig.Name(),
					ParentID:     constants.UserVolumeMountPoint,
					SelinuxLabel: constants.EphemeralSelinuxLabel,
					FileMode:     0o755,
					UID:          0,
					GID:          0,
					BindTarget:   pointer.To(userVolumeConfig.Name()),
				}).
				WriterFunc()

		case block.VolumeTypeDisk:
			userVolumeResource.TransformFunc = NewBuilder().
				WithType(block.VolumeTypeDisk).
				WithDiskLocator(userVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch)).
				WithProvisioning(block.ProvisioningSpec{
					Wave: block.WaveUserVolumes,
					DiskSelector: block.DiskSelector{
						Match: userVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch),
					},
					PartitionSpec: block.PartitionSpec{
						TypeUUID: partition.LinuxFilesystemData,
					},
					FilesystemSpec: block.FilesystemSpec{
						Type: userVolumeConfig.Filesystem().Type(),
					},
				}).
				WithMount(block.MountSpec{
					TargetPath:          userVolumeConfig.Name(),
					ParentID:            constants.UserVolumeMountPoint,
					SelinuxLabel:        constants.EphemeralSelinuxLabel,
					FileMode:            0o755,
					UID:                 0,
					GID:                 0,
					ProjectQuotaSupport: userVolumeConfig.Filesystem().ProjectQuotaSupport(),
				}).
				WithConvertEncryptionConfiguration(userVolumeConfig.Encryption()).
				WriterFunc()

		case block.VolumeTypePartition:
			userVolumeResource.TransformFunc = NewBuilder().
				WithType(block.VolumeTypePartition).
				WithLocator(labelVolumeMatch(volumeID)).
				WithProvisioning(block.ProvisioningSpec{
					Wave: block.WaveUserVolumes,
					DiskSelector: block.DiskSelector{
						Match: userVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch),
					},
					PartitionSpec: block.PartitionSpec{
						MinSize:         cmp.Or(userVolumeConfig.Provisioning().MinSize().ValueOrZero(), MinUserVolumeSize),
						MaxSize:         userVolumeConfig.Provisioning().MaxSize().ValueOrZero(),
						RelativeMaxSize: userVolumeConfig.Provisioning().RelativeMaxSize().ValueOrZero(),
						NegativeMaxSize: userVolumeConfig.Provisioning().MaxSizeNegative(),
						Grow:            userVolumeConfig.Provisioning().Grow().ValueOrZero(),
						Label:           volumeID,
						TypeUUID:        partition.LinuxFilesystemData,
					},
					FilesystemSpec: block.FilesystemSpec{
						Type: userVolumeConfig.Filesystem().Type(),
					},
				}).
				WithMount(block.MountSpec{
					TargetPath:          userVolumeConfig.Name(),
					ParentID:            constants.UserVolumeMountPoint,
					SelinuxLabel:        constants.EphemeralSelinuxLabel,
					FileMode:            0o755,
					UID:                 0,
					GID:                 0,
					ProjectQuotaSupport: userVolumeConfig.Filesystem().ProjectQuotaSupport(),
				}).
				WithConvertEncryptionConfiguration(userVolumeConfig.Encryption()).
				WriterFunc()

		case block.VolumeTypeTmpfs, block.VolumeTypeSymlink, block.VolumeTypeOverlay, block.VolumeTypeExternal:
			fallthrough

		default:
			return nil, fmt.Errorf("unsupported volume type %q", userVolumeConfig.Type().ValueOr(block.VolumeTypePartition).String())
		}

		resources = append(resources, userVolumeResource)
	}

	return resources, nil
}

// RawVolumeTransformer is the transformer for raw volume configs.
func RawVolumeTransformer(c configconfig.Config) ([]VolumeResource, error) {
	if c == nil {
		return nil, nil
	}

	resources := make([]VolumeResource, 0, len(c.RawVolumeConfigs()))

	for _, rawVolumeConfig := range c.RawVolumeConfigs() {
		volumeID := constants.RawVolumePrefix + rawVolumeConfig.Name()
		resources = append(resources, VolumeResource{
			VolumeID: volumeID,
			Label:    block.RawVolumeLabel,
			TransformFunc: NewBuilder().
				WithType(block.VolumeTypePartition).
				WithLocator(labelVolumeMatch(volumeID)).
				WithProvisioning(block.ProvisioningSpec{
					Wave: block.WaveUserVolumes,
					DiskSelector: block.DiskSelector{
						Match: rawVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch),
					},
					PartitionSpec: block.PartitionSpec{
						MinSize:         cmp.Or(rawVolumeConfig.Provisioning().MinSize().ValueOrZero(), MinUserVolumeSize),
						MaxSize:         rawVolumeConfig.Provisioning().MaxSize().ValueOrZero(),
						RelativeMaxSize: rawVolumeConfig.Provisioning().RelativeMaxSize().ValueOrZero(),
						Grow:            rawVolumeConfig.Provisioning().Grow().ValueOrZero(),
						Label:           volumeID,
						TypeUUID:        partition.LinuxFilesystemData,
					},
					FilesystemSpec: block.FilesystemSpec{
						Type: block.FilesystemTypeNone,
					},
				}).
				WithConvertEncryptionConfiguration(rawVolumeConfig.Encryption()).
				WriterFunc(),
			MountTransformFunc: SkipMountTransform,
		})
	}

	return resources, nil
}

// ExistingVolumeTransformer is the transformer for existing user volume configs.
func ExistingVolumeTransformer(c configconfig.Config) ([]VolumeResource, error) {
	if c == nil {
		return nil, nil
	}

	resources := make([]VolumeResource, 0, len(c.ExistingVolumeConfigs()))

	for _, existingVolumeConfig := range c.ExistingVolumeConfigs() {
		volumeID := constants.ExistingVolumePrefix + existingVolumeConfig.Name()

		resources = append(resources, VolumeResource{
			VolumeID: volumeID,
			Label:    block.ExistingVolumeLabel,
			TransformFunc: NewBuilder().
				WithType(block.VolumeTypePartition).
				WithLocator(existingVolumeConfig.VolumeDiscovery().VolumeSelector()).
				WithMount(block.MountSpec{
					TargetPath:   existingVolumeConfig.Name(),
					ParentID:     constants.UserVolumeMountPoint,
					SelinuxLabel: constants.EphemeralSelinuxLabel,
					FileMode:     0o755,
					UID:          0,
					GID:          0,
				}).
				WriterFunc(),
			MountTransformFunc: HandleExistingVolumeMountRequest(existingVolumeConfig),
		})
	}

	return resources, nil
}

// ExternalVolumeTransformer is the transformer for external user volume configs.
func ExternalVolumeTransformer(c configconfig.Config) ([]VolumeResource, error) {
	if c == nil {
		return nil, nil
	}

	resources := make([]VolumeResource, 0, len(c.ExternalVolumeConfigs()))

	for _, externalVolumeConfig := range c.ExternalVolumeConfigs() {
		volumeID := constants.ExternalVolumePrefix + externalVolumeConfig.Name()

		params, err := externalVolumeParameters(externalVolumeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get external volume parameters for volume %q: %w", externalVolumeConfig.Name(), err)
		}

		resources = append(resources, VolumeResource{
			VolumeID: volumeID,
			Label:    block.ExternalVolumeLabel,
			TransformFunc: NewBuilder().
				WithType(block.VolumeTypeExternal).
				WithProvisioning(block.ProvisioningSpec{
					Wave: block.WaveUserVolumes,
					DiskSelector: block.DiskSelector{
						External: externalVolumeSource(externalVolumeConfig),
					},
					FilesystemSpec: block.FilesystemSpec{
						Type: externalVolumeConfig.Type(),
					},
				}).
				WithMount(block.MountSpec{
					TargetPath:   externalVolumeConfig.Name(),
					ParentID:     constants.UserVolumeMountPoint,
					SelinuxLabel: constants.EphemeralSelinuxLabel,
					FileMode:     0o755,
					UID:          0,
					GID:          0,
					Parameters:   params,
				}).
				WriterFunc(),
			MountTransformFunc: HandleExternalVolumeMountRequest(externalVolumeConfig),
		})
	}

	return resources, nil
}

func externalVolumeSource(ext configconfig.ExternalVolumeConfig) string {
	switch ext.Type() {
	case block.FilesystemTypeVirtiofs:
		if ext.Mount().Virtiofs().IsPresent() {
			return ext.Mount().Virtiofs().ValueOrZero().Source()
		}

	case block.FilesystemTypeNone, block.FilesystemTypeXFS, block.FilesystemTypeVFAT, block.FilesystemTypeEXT4, block.FilesystemTypeISO9660, block.FilesystemTypeSwap:
		fallthrough

	default:
		return ""
	}

	return ""
}

func externalVolumeParameters(ext configconfig.ExternalVolumeConfig) ([]block.ParameterSpec, error) {
	switch ext.Type() {
	case block.FilesystemTypeVirtiofs:
		if ext.Mount().Virtiofs().IsPresent() {
			return ext.Mount().Virtiofs().ValueOrZero().Parameters()
		}

		return nil, errors.New("virtiofs mount specification is required for Virtiofs external volume")

	case block.FilesystemTypeNone, block.FilesystemTypeXFS, block.FilesystemTypeVFAT, block.FilesystemTypeEXT4, block.FilesystemTypeISO9660, block.FilesystemTypeSwap:
		fallthrough

	default:
		return nil, fmt.Errorf("unsupported external volume type %q", ext.Type().String())
	}
}

// SwapVolumeTransformer is the transformer for swap volume configs.
func SwapVolumeTransformer(c configconfig.Config) ([]VolumeResource, error) {
	if c == nil {
		return nil, nil
	}

	resources := make([]VolumeResource, 0, len(c.SwapVolumeConfigs()))

	for _, swapVolumeConfig := range c.SwapVolumeConfigs() {
		volumeID := constants.SwapVolumePrefix + swapVolumeConfig.Name()
		resources = append(resources, VolumeResource{
			VolumeID: volumeID,
			Label:    block.SwapVolumeLabel,
			TransformFunc: NewBuilder().
				WithType(block.VolumeTypePartition).
				WithLocator(labelVolumeMatch(volumeID)).
				WithProvisioning(block.ProvisioningSpec{
					Wave: block.WaveUserVolumes,
					DiskSelector: block.DiskSelector{
						Match: swapVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch),
					},
					PartitionSpec: block.PartitionSpec{
						MinSize:         cmp.Or(swapVolumeConfig.Provisioning().MinSize().ValueOrZero(), MinUserVolumeSize),
						MaxSize:         swapVolumeConfig.Provisioning().MaxSize().ValueOrZero(),
						RelativeMaxSize: swapVolumeConfig.Provisioning().RelativeMaxSize().ValueOrZero(),
						NegativeMaxSize: swapVolumeConfig.Provisioning().MaxSizeNegative(),
						Grow:            swapVolumeConfig.Provisioning().Grow().ValueOrZero(),
						Label:           volumeID,
						TypeUUID:        partition.LinkSwap,
					},
					FilesystemSpec: block.FilesystemSpec{
						Type: block.FilesystemTypeSwap,
					},
				}).
				WithConvertEncryptionConfiguration(swapVolumeConfig.Encryption()).
				WriterFunc(),
			MountTransformFunc: DefaultMountTransform,
		})
	}

	return resources, nil
}

// HandleUserVolumeMountRequest returns a MountTransformFunc for user volumes.
func HandleUserVolumeMountRequest(userVolumeConfig configconfig.UserVolumeConfig) func(m *block.VolumeMountRequest) error {
	return func(m *block.VolumeMountRequest) error {
		m.TypedSpec().DisableAccessTime = userVolumeConfig.Mount().DisableAccessTime()
		m.TypedSpec().Secure = userVolumeConfig.Mount().Secure()

		return nil
	}
}

// HandleExistingVolumeMountRequest returns a MountTransformFunc for existing volumes.
// It sets `VolumeMountRequestSpec.ReadOnly` based on the existing configuration.
func HandleExistingVolumeMountRequest(existingVolumeConfig configconfig.ExistingVolumeConfig) func(m *block.VolumeMountRequest) error {
	return func(m *block.VolumeMountRequest) error {
		m.TypedSpec().ReadOnly = existingVolumeConfig.Mount().ReadOnly()
		m.TypedSpec().DisableAccessTime = existingVolumeConfig.Mount().DisableAccessTime()
		m.TypedSpec().Secure = existingVolumeConfig.Mount().Secure()

		return nil
	}
}

// HandleExternalVolumeMountRequest returns a MountTransformFunc for external volumes.
func HandleExternalVolumeMountRequest(externalVolumeConfig configconfig.ExternalVolumeConfig) func(m *block.VolumeMountRequest) error {
	return func(m *block.VolumeMountRequest) error {
		m.TypedSpec().ReadOnly = externalVolumeConfig.Mount().ReadOnly()
		m.TypedSpec().DisableAccessTime = externalVolumeConfig.Mount().DisableAccessTime()
		m.TypedSpec().Secure = externalVolumeConfig.Mount().Secure()

		return nil
	}
}

// DefaultMountTransform is a no-op.
func DefaultMountTransform(_ *block.VolumeMountRequest) error {
	return nil
}

// SkipMountTransform is a MountTransformFunc which skips creating a MountRequest.
// It returns a tagged error, which is handled by the VolumeConfigController.
func SkipMountTransform(_ *block.VolumeMountRequest) error {
	return xerrors.NewTaggedf[SkipUserVolumeMountRequest]("skip")
}
