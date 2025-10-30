// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"cmp"
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

var userVolumeTransformers = []volumeConfigTransformer{
	userVolumeTransformer,
	rawVolumeTransformer,
	existingVolumeTransformer,
	swapVolumeTransformer,
}

var (
	userVolumeTransformer = func(c configconfig.Config) ([]volumeResource, error) {
		if c == nil {
			return nil, nil
		}

		var resources []volumeResource
		for _, userVolumeConfig := range c.UserVolumeConfigs() {
			volumeID := constants.UserVolumePrefix + userVolumeConfig.Name()
			userVolumeResource := volumeResource{
				VolumeID:           volumeID,
				Label:              block.UserVolumeLabel,
				MountTransformFunc: defaultMountTransform,
			}

			switch userVolumeConfig.Type().ValueOr(block.VolumeTypePartition) {
			case block.VolumeTypeDirectory:
				userVolumeResource.TransformFunc = newVolumeConfigBuilder().
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
				userVolumeResource.TransformFunc = newVolumeConfigBuilder().
					WithType(block.VolumeTypeDisk).
					WithLocator(userVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch)).
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
				userVolumeResource.TransformFunc = newVolumeConfigBuilder().
					WithType(block.VolumeTypePartition).
					WithLocator(labelVolumeMatch(volumeID)).
					WithProvisioning(block.ProvisioningSpec{
						Wave: block.WaveUserVolumes,
						DiskSelector: block.DiskSelector{
							Match: userVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch),
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

			case block.VolumeTypeTmpfs, block.VolumeTypeSymlink, block.VolumeTypeOverlay:
				fallthrough

			default:
				return nil, fmt.Errorf("unsupported volume type %q", userVolumeConfig.Type().ValueOr(block.VolumeTypePartition).String())
			}

			resources = append(resources, userVolumeResource)
		}

		return resources, nil
	}

	rawVolumeTransformer = func(c configconfig.Config) ([]volumeResource, error) {
		if c == nil {
			return nil, nil
		}

		var resources []volumeResource
		for _, rawVolumeConfig := range c.RawVolumeConfigs() {
			volumeID := constants.RawVolumePrefix + rawVolumeConfig.Name()
			resources = append(resources, volumeResource{
				VolumeID: volumeID,
				Label:    block.RawVolumeLabel,
				TransformFunc: newVolumeConfigBuilder().
					WithType(block.VolumeTypePartition).
					WithLocator(labelVolumeMatch(volumeID)).
					WithProvisioning(block.ProvisioningSpec{
						Wave: block.WaveUserVolumes,
						DiskSelector: block.DiskSelector{
							Match: rawVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch),
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
					}).
					WithConvertEncryptionConfiguration(rawVolumeConfig.Encryption()).
					WriterFunc(),
				MountTransformFunc: skipMountTransform,
			})
		}

		return resources, nil
	}

	existingVolumeTransformer = func(c configconfig.Config) ([]volumeResource, error) {
		if c == nil {
			return nil, nil
		}

		var resources []volumeResource
		for _, existingVolumeConfig := range c.ExistingVolumeConfigs() {
			volumeID := constants.ExistingVolumePrefix + existingVolumeConfig.Name()
			resources = append(resources, volumeResource{
				VolumeID: volumeID,
				Label:    block.ExistingVolumeLabel,
				TransformFunc: newVolumeConfigBuilder().
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
				MountTransformFunc: handleExistingVolumeMountRequest(existingVolumeConfig),
			})
		}

		return resources, nil
	}

	swapVolumeTransformer = func(c configconfig.Config) ([]volumeResource, error) {
		if c == nil {
			return nil, nil
		}

		var resources []volumeResource
		for _, swapVolumeConfig := range c.SwapVolumeConfigs() {
			volumeID := constants.SwapVolumePrefix + swapVolumeConfig.Name()
			resources = append(resources, volumeResource{
				VolumeID: volumeID,
				Label:    block.SwapVolumeLabel,
				TransformFunc: newVolumeConfigBuilder().
					WithType(block.VolumeTypePartition).
					WithLocator(labelVolumeMatch(volumeID)).
					WithProvisioning(block.ProvisioningSpec{
						Wave: block.WaveUserVolumes,
						DiskSelector: block.DiskSelector{
							Match: swapVolumeConfig.Provisioning().DiskSelector().ValueOr(noMatch),
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
					}).
					WithConvertEncryptionConfiguration(swapVolumeConfig.Encryption()).
					WriterFunc(),
				MountTransformFunc: defaultMountTransform,
			})
		}

		return resources, nil
	}

	handleExistingVolumeMountRequest = func(existingVolumeConfig configconfig.ExistingVolumeConfig) func(m *block.VolumeMountRequest) error {
		return func(m *block.VolumeMountRequest) error {
			m.TypedSpec().ReadOnly = existingVolumeConfig.Mount().ReadOnly()

			return nil
		}
	}

	defaultMountTransform = func(_ *block.VolumeMountRequest) error {
		return nil
	}

	skipMountTransform = func(_ *block.VolumeMountRequest) error {
		return xerrors.NewTaggedf[skipUserVolumeMountRequest]("skip")
	}
)

// skipUserVolumeMountRequest is used to skip creating a VolumeMountRequest for a user volume.
type skipUserVolumeMountRequest struct{}
