// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/partition"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func (ctrl *VolumeConfigController) getSystemVolumeTransformers(ctx context.Context, encryptionMeta *runtime.MetaKey, l *zap.Logger) []volumeConfigTransformer { //nolint:gocyclo
	metaVolumeTransformer := func(_ configconfig.Config) ([]volumeResource, error) {
		return []volumeResource{
			{
				VolumeID: constants.MetaPartitionLabel,
				Label:    block.SystemVolumeLabel,
				TransformFunc: newVolumeConfigBuilder().
					WithType(block.VolumeTypePartition).
					WithLocator(metaMatch()).
					WriterFunc(),
			},
		}, nil
	}

	stateVolumeTransformer := func(cfg configconfig.Config) ([]volumeResource, error) {
		var volumeConfigurator func(vc *block.VolumeConfig) error

		if ctrl.V1Alpha1Mode.InContainer() {
			volumeConfigurator = newVolumeConfigBuilder().
				WithType(block.VolumeTypeDirectory).
				WithMount(block.MountSpec{
					TargetPath:   constants.StateMountPoint,
					SelinuxLabel: constants.StateSelinuxLabel,
					FileMode:     0o700,
					UID:          0,
					GID:          0,
				}).WriterFunc()
		} else {
			// STATE configuration should be always created, but it depends on the configuration presence
			if cfg != nil && cfg.Machine() != nil {
				volumeConfigurator = ctrl.manageStateConfigPresent(ctx, l, cfg)
			} else {
				volumeConfigurator = ctrl.manageStateNoConfig(encryptionMeta)
			}
		}

		return []volumeResource{
			{
				VolumeID:      constants.StatePartitionLabel,
				Label:         block.SystemVolumeLabel,
				TransformFunc: volumeConfigurator,
			},
		}, nil
	}

	ephemeralVolumeTransformer := func(cfg configconfig.Config) ([]volumeResource, error) {
		// skip if no config
		if cfg == nil || cfg.Machine() == nil {
			return nil, nil
		}

		var volumeConfigurator func(*block.VolumeConfig) error

		if ctrl.V1Alpha1Mode.InContainer() {
			volumeConfigurator = newVolumeConfigBuilder().
				WithType(block.VolumeTypeDirectory).
				WithMount(block.MountSpec{
					TargetPath:   constants.EphemeralMountPoint,
					SelinuxLabel: constants.EphemeralSelinuxLabel,
					FileMode:     0o755,
					UID:          0,
					GID:          0,
				}).WriterFunc()
		} else {
			volumeConfigurator = func(vc *block.VolumeConfig) error {
				extraVolumeConfig, _ := cfg.Volumes().ByName(constants.EphemeralPartitionLabel)

				return newVolumeConfigBuilder().
					WithType(block.VolumeTypePartition).
					WithProvisioning(block.ProvisioningSpec{
						Wave: block.WaveSystemDisk,
						DiskSelector: block.DiskSelector{
							Match: extraVolumeConfig.Provisioning().DiskSelector().ValueOr(systemDiskMatch()),
						},
						PartitionSpec: block.PartitionSpec{
							MinSize:  extraVolumeConfig.Provisioning().MinSize().ValueOr(quirks.New("").PartitionSizes().EphemeralMinSize()),
							MaxSize:  extraVolumeConfig.Provisioning().MaxSize().ValueOr(0),
							Grow:     extraVolumeConfig.Provisioning().Grow().ValueOr(true),
							Label:    constants.EphemeralPartitionLabel,
							TypeUUID: partition.LinuxFilesystemData,
						},
						FilesystemSpec: block.FilesystemSpec{
							Type:  block.FilesystemTypeXFS,
							Label: constants.EphemeralPartitionLabel,
						},
					}).
					WithMount(block.MountSpec{
						TargetPath:          constants.EphemeralMountPoint,
						SelinuxLabel:        constants.EphemeralSelinuxLabel,
						FileMode:            0o755,
						UID:                 0,
						GID:                 0,
						ProjectQuotaSupport: cfg.Machine().Features().DiskQuotaSupportEnabled(),
					}).
					WithLocator(labelVolumeMatch(constants.EphemeralPartitionLabel)).
					WithFunc(func(vcs *block.VolumeConfigSpec) error {
						encryptionConfig := extraVolumeConfig.Encryption()
						if encryptionConfig == nil {
							// fall back to v1alpha1 encryption config
							encryptionConfig = cfg.Machine().SystemDiskEncryption().Get(constants.EphemeralPartitionLabel)
						}

						if err := convertEncryptionConfiguration(
							encryptionConfig,
							vc.TypedSpec(),
						); err != nil {
							return fmt.Errorf("error converting encryption for %s: %w", constants.EphemeralPartitionLabel, err)
						}

						return nil
					}).
					Apply(vc.TypedSpec())
			}
		}

		return []volumeResource{{
			VolumeID:      constants.EphemeralPartitionLabel,
			Label:         block.SystemVolumeLabel,
			TransformFunc: volumeConfigurator,
		}}, nil
	}

	standardDirectoryVolumesTransformer := func(cfg configconfig.Config) ([]volumeResource, error) {
		// skip if no config
		if cfg == nil || cfg.Machine() == nil {
			return nil, nil
		}

		resources := []volumeResource{
			// /var/run symlink
			{
				VolumeID: "/var/run",
				Label:    block.SystemVolumeLabel,
				TransformFunc: newVolumeConfigBuilder().
					WithType(block.VolumeTypeSymlink).
					WithSymlink(block.SymlinkProvisioningSpec{
						SymlinkTargetPath: "/run",
						Force:             true,
					}).
					WithMount(block.MountSpec{
						TargetPath: "/var/run",
					}).
					WriterFunc(),
			},
		}

		parentIDs := map[string]string{
			"/var":     constants.EphemeralPartitionLabel,
			"/var/run": "/var/run",
		}

		for _, volume := range standardVolumeDefinitions {
			parentDir := filepath.Dir(volume.Path)
			targetDir := filepath.Base(volume.Path)

			parentID, ok := parentIDs[parentDir]
			if !ok {
				return nil, fmt.Errorf("unknown parent directory volume %q for %q", parentDir, volume.Path)
			}

			volumeID := volume.Path
			if volume.ID != "" {
				volumeID = volume.ID
			}

			resources = append(resources, volumeResource{
				VolumeID: volumeID,
				Label:    block.SystemVolumeLabel,
				TransformFunc: newVolumeConfigBuilder().
					WithType(block.VolumeTypeDirectory).
					WithMount(block.MountSpec{
						TargetPath:       targetDir,
						ParentID:         parentID,
						SelinuxLabel:     volume.SELinuxLabel,
						FileMode:         volume.Mode,
						UID:              volume.UID,
						GID:              volume.GID,
						RecursiveRelabel: volume.Recursive,
					}).WriterFunc(),
			})

			parentIDs[volume.Path] = volumeID
		}

		return resources, nil
	}

	overlayVolumesTransformer := func(cfg configconfig.Config) ([]volumeResource, error) {
		// skip if no config or in container
		if cfg == nil || cfg.Machine() == nil || ctrl.V1Alpha1Mode.InContainer() {
			return nil, nil
		}

		var resources []volumeResource
		for _, overlay := range constants.Overlays {
			resources = append(resources, volumeResource{
				VolumeID: overlay.Path,
				Label:    block.SystemVolumeLabel,
				TransformFunc: newVolumeConfigBuilder().
					WithType(block.VolumeTypeOverlay).
					WithParentID(constants.EphemeralPartitionLabel).
					WithMount(block.MountSpec{
						TargetPath:   overlay.Path,
						SelinuxLabel: overlay.Label,
						FileMode:     0o755,
						UID:          0,
						GID:          0,
					}).WriterFunc(),
			})
		}

		return resources, nil
	}

	return []volumeConfigTransformer{
		metaVolumeTransformer,
		stateVolumeTransformer,
		ephemeralVolumeTransformer,
		standardDirectoryVolumesTransformer,
		overlayVolumesTransformer,
	}
}

func (ctrl *VolumeConfigController) manageStateNoConfig(encryptionMeta *runtime.MetaKey) func(vc *block.VolumeConfig) error {
	match := labelVolumeMatchAndNonEmpty(constants.StatePartitionLabel)
	if ctrl.V1Alpha1Mode.IsAgent() { // mark as missing
		match = noMatch
	}

	return newVolumeConfigBuilder().
		WithType(block.VolumeTypePartition).
		WithMount(block.MountSpec{
			TargetPath:   constants.StateMountPoint,
			SelinuxLabel: constants.StateSelinuxLabel,
			FileMode:     0o700,
			UID:          0,
			GID:          0,
		}).WithLocator(match).
		WithFunc(func(spec *block.VolumeConfigSpec) error {
			if encryptionMeta != nil {
				encryptionFromMeta, err := UnmarshalEncryptionMeta([]byte(encryptionMeta.TypedSpec().Value))
				if err != nil {
					return err
				}

				if err := convertEncryptionConfiguration(
					encryptionFromMeta,
					spec,
				); err != nil {
					return fmt.Errorf("error converting encryption for %s: %w", constants.StatePartitionLabel, err)
				}
			} else {
				spec.Encryption = block.EncryptionSpec{}
			}

			return nil
		}).
		WriterFunc()
}

func (ctrl *VolumeConfigController) manageStateConfigPresent(ctx context.Context, l *zap.Logger, cfg configconfig.Config) func(vc *block.VolumeConfig) error {
	return func(vc *block.VolumeConfig) error {
		extraVolumeConfig, _ := cfg.Volumes().ByName(constants.StatePartitionLabel)

		return newVolumeConfigBuilder().
			WithType(block.VolumeTypePartition).
			WithMount(block.MountSpec{
				TargetPath:   constants.StateMountPoint,
				SelinuxLabel: constants.StateSelinuxLabel,
				FileMode:     0o700,
				UID:          0,
				GID:          0,
			}).
			WithProvisioning(block.ProvisioningSpec{
				Wave: block.WaveSystemDisk,
				DiskSelector: block.DiskSelector{
					Match: systemDiskMatch(),
				},
				PartitionSpec: block.PartitionSpec{
					MinSize:  quirks.New("").PartitionSizes().StateSize(),
					MaxSize:  quirks.New("").PartitionSizes().StateSize(),
					Label:    constants.StatePartitionLabel,
					TypeUUID: partition.LinuxFilesystemData,
				},
				FilesystemSpec: block.FilesystemSpec{
					Type:  block.FilesystemTypeXFS,
					Label: constants.StatePartitionLabel,
				},
			}).
			WithLocator(labelVolumeMatch(constants.StatePartitionLabel)).
			WithFunc(func(spec *block.VolumeConfigSpec) error {
				encryptionConfig := extraVolumeConfig.Encryption()
				if encryptionConfig == nil {
					// fall back to v1alpha1 encryption config
					encryptionConfig = cfg.Machine().SystemDiskEncryption().Get(constants.StatePartitionLabel)
				}

				if err := convertEncryptionConfiguration(
					encryptionConfig,
					vc.TypedSpec(),
				); err != nil {
					return fmt.Errorf("error converting encryption for %s: %w", constants.StatePartitionLabel, err)
				}

				metaEncryptionConfig, err := MarshalEncryptionMeta(encryptionConfig)
				if err != nil {
					return fmt.Errorf("error marshaling encryption config for %s: %w", constants.StatePartitionLabel, err)
				}

				previous, ok := ctrl.MetaProvider.Meta().ReadTagBytes(meta.StateEncryptionConfig)
				if ok && bytes.Equal(previous, metaEncryptionConfig) {
					return nil
				}

				ok, err = ctrl.MetaProvider.Meta().SetTagBytes(ctx, meta.StateEncryptionConfig, metaEncryptionConfig)
				if err != nil {
					return fmt.Errorf("error setting meta tag %q: %w", meta.StateEncryptionConfig, err)
				}

				if !ok {
					return errors.New("failed to save state encryption config to meta")
				}

				if err = ctrl.MetaProvider.Meta().Flush(); err != nil {
					return fmt.Errorf("error flushing meta: %w", err)
				}

				l.Info("saved state encryption config to META")

				return nil
			}).
			Apply(vc.TypedSpec())
	}
}

var standardVolumeDefinitions = []struct {
	ID           string
	Path         string
	Mode         os.FileMode
	UID          int
	GID          int
	Recursive    bool
	SELinuxLabel string
}{
	// /var/log
	{
		Path:         "/var/log",
		Mode:         0o755,
		SELinuxLabel: "system_u:object_r:var_log_t:s0",
	},
	{
		Path:         "/var/log/audit",
		Mode:         0o700,
		SELinuxLabel: "system_u:object_r:audit_log_t:s0",
	},
	{
		Path:         constants.KubernetesAuditLogDir,
		Mode:         0o700,
		UID:          constants.KubernetesAPIServerRunUser,
		GID:          constants.KubernetesAPIServerRunGroup,
		Recursive:    true,
		SELinuxLabel: "system_u:object_r:kube_log_t:s0",
	},
	{
		Path:         "/var/log/containers",
		Mode:         0o755,
		SELinuxLabel: "system_u:object_r:containers_log_t:s0",
	},
	{
		Path:         "/var/log/pods",
		Mode:         0o755,
		SELinuxLabel: "system_u:object_r:pods_log_t:s0",
	},
	// /var/lib
	{
		Path:         "/var/lib",
		Mode:         0o700,
		SELinuxLabel: constants.EphemeralSelinuxLabel,
	},
	{
		ID:           constants.EtcdDataVolumeID,
		Path:         constants.EtcdDataPath,
		SELinuxLabel: constants.EtcdDataSELinuxLabel,
		Mode:         0o700,
		UID:          constants.EtcdUserID,
		GID:          constants.EtcdUserID,
		Recursive:    true,
	},
	{
		Path:         "/var/lib/containerd",
		Mode:         0o000,
		SELinuxLabel: "system_u:object_r:containerd_state_t:s0",
	},
	{
		Path:         "/var/lib/kubelet",
		Mode:         0o700,
		SELinuxLabel: "system_u:object_r:kubelet_state_t:s0",
	},
	{
		Path:         "/var/lib/cni",
		Mode:         0o700,
		Recursive:    true,
		SELinuxLabel: "system_u:object_r:cni_state_t:s0",
	},
	{
		Path:         "/var/lib/kubelet/seccomp",
		Mode:         0o700,
		SELinuxLabel: "system_u:object_r:seccomp_profile_t:s0",
	},
	{
		Path:         constants.SeccompProfilesDirectory,
		Mode:         0o700,
		Recursive:    true,
		SELinuxLabel: "system_u:object_r:seccomp_profile_t:s0",
	},
	// /var/mnt
	{
		Path:         constants.UserVolumeMountPoint,
		Mode:         0o755,
		SELinuxLabel: constants.EphemeralSelinuxLabel,
	},
	// /var/run
	{
		Path:         "/var/run/lock",
		Mode:         0o755,
		SELinuxLabel: "system_u:object_r:var_lock_t:s0",
	},
}
