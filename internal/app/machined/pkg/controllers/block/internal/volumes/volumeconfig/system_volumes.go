// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package volumeconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	"github.com/siderolabs/talos/internal/pkg/partition"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// GetSystemVolumeTransformers returns the transformers for system volumes.
func GetSystemVolumeTransformers(ctx context.Context,
	encryptionMeta *runtime.MetaKey,
	inContainer, isAgent bool,
) []volumeConfigTransformer { //nolint:gocyclo
	metaVolumeTransformer := func(_ configconfig.Config) ([]VolumeResource, error) {
		return []VolumeResource{
			{
				VolumeID: constants.MetaPartitionLabel,
				Label:    block.SystemVolumeLabel,
				TransformFunc: NewBuilder().
					WithType(block.VolumeTypePartition).
					WithLocator(metaMatch()).
					WriterFunc(),
			},
		}, nil
	}

	return []volumeConfigTransformer{
		metaVolumeTransformer,
		GetStateVolumeTransformer(encryptionMeta, inContainer, isAgent),
		GetEphemeralVolumeTransformer(inContainer),
		StandardDirectoryVolumesTransformer,
		GetOverlayVolumesTransformer(inContainer),
	}
}

// GetStateVolumeTransformer returns the transformer for the STATE volume.
func GetStateVolumeTransformer(encryptionMeta *runtime.MetaKey, inContainer, isAgent bool) volumeConfigTransformer {
	return func(cfg configconfig.Config) ([]VolumeResource, error) {
		var volumeConfigurator func(*block.VolumeConfig) error

		if inContainer {
			volumeConfigurator = NewBuilder().
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
				volumeConfigurator = manageStateConfigPresent(cfg)
			} else {
				volumeConfigurator = manageStateNoConfig(encryptionMeta, isAgent)
			}
		}

		return []VolumeResource{
			{
				VolumeID:      constants.StatePartitionLabel,
				Label:         block.SystemVolumeLabel,
				TransformFunc: volumeConfigurator,
			},
		}, nil
	}
}

// GetEphemeralVolumeTransformer returns the transformer for the EPHEMERAL volume.
func GetEphemeralVolumeTransformer(inContainer bool) volumeConfigTransformer {
	return func(cfg configconfig.Config) ([]VolumeResource, error) {
		// skip if no config
		if cfg == nil || cfg.Machine() == nil {
			return nil, nil
		}

		var volumeConfigurator func(*block.VolumeConfig) error

		if inContainer {
			volumeConfigurator = NewBuilder().
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

				return NewBuilder().
					WithType(block.VolumeTypePartition).
					WithProvisioning(block.ProvisioningSpec{
						Wave: block.WaveSystemDisk,
						DiskSelector: block.DiskSelector{
							Match: extraVolumeConfig.Provisioning().DiskSelector().ValueOr(systemDiskMatch()),
						},
						PartitionSpec: block.PartitionSpec{
							MinSize:         extraVolumeConfig.Provisioning().MinSize().ValueOr(quirks.New("").PartitionSizes().EphemeralMinSize()),
							MaxSize:         extraVolumeConfig.Provisioning().MaxSize().ValueOrZero(),
							RelativeMaxSize: extraVolumeConfig.Provisioning().RelativeMaxSize().ValueOrZero(),
							NegativeMaxSize: extraVolumeConfig.Provisioning().MaxSizeNegative(),
							Grow:            extraVolumeConfig.Provisioning().Grow().ValueOr(true),
							Label:           constants.EphemeralPartitionLabel,
							TypeUUID:        partition.LinuxFilesystemData,
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

						if err := volumes.ConvertEncryptionConfiguration(
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

		return []VolumeResource{{
			VolumeID:      constants.EphemeralPartitionLabel,
			Label:         block.SystemVolumeLabel,
			TransformFunc: volumeConfigurator,
		}}, nil
	}
}

// GetOverlayVolumesTransformer returns the transformer for overlay volumes.
func GetOverlayVolumesTransformer(inContainer bool) func(configconfig.Config) ([]VolumeResource, error) {
	return func(cfg configconfig.Config) ([]VolumeResource, error) {
		// skip if no config or in container
		if cfg == nil || cfg.Machine() == nil || inContainer {
			return nil, nil
		}

		var resources []VolumeResource
		for _, overlay := range constants.Overlays {
			resources = append(resources, VolumeResource{
				VolumeID: overlay.Path,
				Label:    block.SystemVolumeLabel,
				TransformFunc: NewBuilder().
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
}

func manageStateNoConfig(encryptionMeta *runtime.MetaKey, isAgent bool) func(vc *block.VolumeConfig) error {
	match := labelVolumeMatchAndNonEmpty(constants.StatePartitionLabel)
	if isAgent { // mark as missing
		match = noMatch
	}

	return NewBuilder().
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
				encryptionFromMeta, err := volumes.UnmarshalEncryptionMeta([]byte(encryptionMeta.TypedSpec().Value))
				if err != nil {
					return err
				}

				if err := volumes.ConvertEncryptionConfiguration(
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

func manageStateConfigPresent(cfg configconfig.Config) func(vc *block.VolumeConfig) error {
	return func(vc *block.VolumeConfig) error {
		extraVolumeConfig, _ := cfg.Volumes().ByName(constants.StatePartitionLabel)

		encryptionConfig := extraVolumeConfig.Encryption()
		if encryptionConfig == nil {
			// fall back to v1alpha1 encryption config
			encryptionConfig = cfg.Machine().SystemDiskEncryption().Get(constants.StatePartitionLabel)
		}

		return NewBuilder().
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
			WithConvertEncryptionConfiguration(encryptionConfig).
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

// StandardDirectoryVolumesTransformer is the transformer for standard directory volumes,
// including the /var/run symlink.
func StandardDirectoryVolumesTransformer(cfg configconfig.Config) ([]VolumeResource, error) {
	// skip if no config
	if cfg == nil || cfg.Machine() == nil {
		return nil, nil
	}

	resources := []VolumeResource{
		// /var/run symlink
		{
			VolumeID: "/var/run",
			Label:    block.SystemVolumeLabel,
			TransformFunc: NewBuilder().
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

		resources = append(resources, VolumeResource{
			VolumeID: volumeID,
			Label:    block.SystemVolumeLabel,
			TransformFunc: NewBuilder().
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
