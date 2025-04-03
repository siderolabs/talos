// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"go.uber.org/zap"

	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	cfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var noMatch = cel.MustExpression(cel.ParseBooleanExpression("false", celenv.Empty()))

// VolumeConfigController provides volume configuration based on Talos defaults and machine configuration.
type VolumeConfigController struct {
	V1Alpha1Mode machinedruntime.Mode
}

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
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaKeyType,
			ID:        optional.Some(runtime.MetaKeyTagToID(meta.StateEncryptionConfig)),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VolumeConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeConfigType,
			Kind: controller.OutputShared,
		},
	}
}

func labelVolumeMatch(label string) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s'", label), celenv.VolumeLocator()))
}

func labelVolumeMatchAndNonEmpty(label string) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s' && volume.name != ''", label), celenv.VolumeLocator()))
}

func metaMatch() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s' && volume.name in ['', 'talosmeta'] && volume.size == 1048576u", constants.MetaPartitionLabel), celenv.VolumeLocator())) //nolint:lll
}

func systemDiskMatch() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression("system_disk", celenv.DiskLocator()))
}

func (ctrl *VolumeConfigController) convertEncryption(in cfg.Encryption, out *block.VolumeConfigSpec) error {
	if in == nil {
		out.Encryption = block.EncryptionSpec{}

		return nil
	}

	switch in.Provider() {
	case encryption.LUKS2:
		out.Encryption.Provider = block.EncryptionProviderLUKS2
	default:
		return fmt.Errorf("unsupported encryption provider: %s", in.Provider())
	}

	out.Encryption.Cipher = in.Cipher()
	out.Encryption.KeySize = in.KeySize()
	out.Encryption.BlockSize = in.BlockSize()
	out.Encryption.PerfOptions = in.Options()

	out.Encryption.Keys = make([]block.EncryptionKey, len(in.Keys()))

	for i, key := range in.Keys() {
		out.Encryption.Keys[i].Slot = key.Slot()

		switch {
		case key.Static() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyStatic
			out.Encryption.Keys[i].StaticPassphrase = key.Static().Key()
		case key.NodeID() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyNodeID
		case key.KMS() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyKMS
			out.Encryption.Keys[i].KMSEndpoint = key.KMS().Endpoint()
		case key.TPM() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyTPM
			out.Encryption.Keys[i].TPMCheckSecurebootStatusOnEnroll = key.TPM().CheckSecurebootOnEnroll()
		default:
			return fmt.Errorf("unsupported encryption key type: slot %d", key.Slot())
		}
	}

	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *VolumeConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		// load config if present
		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching machine configuration")
		}

		// load STATE encryption meta key
		encryptionMeta, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.StateEncryptionConfig))
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error fetching state encryption meta key")
		}

		r.StartTrackingOutputs()

		// META volume discovery, always created unconditionally
		// META volume is created by the installer, and never by Talos running on the machine
		if err = safe.WriterModify(ctx, r,
			block.NewVolumeConfig(block.NamespaceName, constants.MetaPartitionLabel),
			func(vc *block.VolumeConfig) error {
				vc.TypedSpec().Type = block.VolumeTypePartition
				vc.TypedSpec().Locator = block.LocatorSpec{
					Match: metaMatch(),
				}

				return nil
			},
		); err != nil {
			return fmt.Errorf("error creating meta volume configuration: %w", err)
		}

		// if config is present (v1apha1 part of now)
		// [TODO]: support custom configuration later
		configurationPresent := cfg != nil && cfg.Config().Machine() != nil

		// STATE configuration should be always created, but it depends on the configuration presence
		if configurationPresent {
			err = safe.WriterModify(ctx, r,
				block.NewVolumeConfig(block.NamespaceName, constants.StatePartitionLabel),
				ctrl.manageStateConfigPresent(cfg.Config()),
			)
		} else {
			err = safe.WriterModify(ctx, r,
				block.NewVolumeConfig(block.NamespaceName, constants.StatePartitionLabel),
				ctrl.manageStateNoConfig(encryptionMeta),
			)
		}

		if err != nil {
			return fmt.Errorf("error creating state volume configuration: %w", err)
		}

		if configurationPresent {
			if err = safe.WriterModify(ctx, r,
				block.NewVolumeConfig(block.NamespaceName, constants.EphemeralPartitionLabel),
				ctrl.manageEphemeral(cfg.Config()),
			); err != nil {
				return fmt.Errorf("error creating ephemeral volume configuration: %w", err)
			}

			if err = ctrl.manageStandardVolumes(ctx, r); err != nil {
				return fmt.Errorf("error creating standard volume configuration: %w", err)
			}

			if err = ctrl.manageOverlayVolumes(ctx, r); err != nil {
				return fmt.Errorf("error creating overlay volume configuration: %w", err)
			}
		}

		// [TODO]: this would fail as it doesn't handle finalizers properly
		if err = safe.CleanupOutputs[*block.VolumeConfig](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up volume configuration: %w", err)
		}
	}
}

func (ctrl *VolumeConfigController) manageEphemeralInContainer(vc *block.VolumeConfig) error {
	vc.TypedSpec().Type = block.VolumeTypeDirectory
	vc.TypedSpec().Mount = block.MountSpec{
		TargetPath:   constants.EphemeralMountPoint,
		SelinuxLabel: constants.EphemeralSelinuxLabel,
		FileMode:     0o755,
		UID:          0,
		GID:          0,
	}

	return nil
}

func (ctrl *VolumeConfigController) manageEphemeral(config cfg.Config) func(vc *block.VolumeConfig) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return ctrl.manageEphemeralInContainer
	}

	return func(vc *block.VolumeConfig) error {
		extraVolumeConfig, _ := config.Volumes().ByName(constants.EphemeralPartitionLabel)

		vc.TypedSpec().Type = block.VolumeTypePartition

		vc.TypedSpec().Provisioning = block.ProvisioningSpec{
			Wave: block.WaveSystemDisk,
			DiskSelector: block.DiskSelector{
				Match: extraVolumeConfig.Provisioning().DiskSelector().ValueOr(systemDiskMatch()),
			},
			PartitionSpec: block.PartitionSpec{
				MinSize:  extraVolumeConfig.Provisioning().MinSize().ValueOr(partition.EphemeralMinSize),
				MaxSize:  extraVolumeConfig.Provisioning().MaxSize().ValueOr(0),
				Grow:     extraVolumeConfig.Provisioning().Grow().ValueOr(true),
				Label:    constants.EphemeralPartitionLabel,
				TypeUUID: partition.LinuxFilesystemData,
			},
			FilesystemSpec: block.FilesystemSpec{
				Type:  block.FilesystemTypeXFS,
				Label: constants.EphemeralPartitionLabel,
			},
		}

		vc.TypedSpec().Mount = block.MountSpec{
			TargetPath:          constants.EphemeralMountPoint,
			SelinuxLabel:        constants.EphemeralSelinuxLabel,
			FileMode:            0o755,
			UID:                 0,
			GID:                 0,
			ProjectQuotaSupport: config.Machine().Features().DiskQuotaSupportEnabled(),
		}

		vc.TypedSpec().Locator = block.LocatorSpec{
			Match: labelVolumeMatch(constants.EphemeralPartitionLabel),
		}

		if err := ctrl.convertEncryption(
			config.Machine().SystemDiskEncryption().Get(constants.EphemeralPartitionLabel),
			vc.TypedSpec(),
		); err != nil {
			return fmt.Errorf("error converting encryption for %s: %w", constants.EphemeralPartitionLabel, err)
		}

		return nil
	}
}

func (ctrl *VolumeConfigController) manageStateInContainer(vc *block.VolumeConfig) error {
	vc.TypedSpec().Type = block.VolumeTypeDirectory
	vc.TypedSpec().Mount = block.MountSpec{
		TargetPath:   constants.StateMountPoint,
		SelinuxLabel: constants.StateSelinuxLabel,
		FileMode:     0o700,
		UID:          0,
		GID:          0,
	}

	return nil
}

func (ctrl *VolumeConfigController) manageStateConfigPresent(config cfg.Config) func(vc *block.VolumeConfig) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return ctrl.manageStateInContainer
	}

	return func(vc *block.VolumeConfig) error {
		vc.TypedSpec().Type = block.VolumeTypePartition
		vc.TypedSpec().Mount = block.MountSpec{
			TargetPath:   constants.StateMountPoint,
			SelinuxLabel: constants.StateSelinuxLabel,
			FileMode:     0o700,
			UID:          0,
			GID:          0,
		}

		vc.TypedSpec().Provisioning = block.ProvisioningSpec{
			Wave: block.WaveSystemDisk,
			DiskSelector: block.DiskSelector{
				Match: systemDiskMatch(),
			},
			PartitionSpec: block.PartitionSpec{
				MinSize:  partition.StateSize,
				MaxSize:  partition.StateSize,
				Label:    constants.StatePartitionLabel,
				TypeUUID: partition.LinuxFilesystemData,
			},
			FilesystemSpec: block.FilesystemSpec{
				Type:  block.FilesystemTypeXFS,
				Label: constants.StatePartitionLabel,
			},
		}

		vc.TypedSpec().Locator = block.LocatorSpec{
			Match: labelVolumeMatch(constants.StatePartitionLabel),
		}

		if err := ctrl.convertEncryption(
			config.Machine().SystemDiskEncryption().Get(constants.StatePartitionLabel),
			vc.TypedSpec(),
		); err != nil {
			return fmt.Errorf("error converting encryption for %s: %w", constants.StatePartitionLabel, err)
		}

		return nil
	}
}

func (ctrl *VolumeConfigController) manageStateNoConfig(encryptionMeta *runtime.MetaKey) func(vc *block.VolumeConfig) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return ctrl.manageStateInContainer
	}

	return func(vc *block.VolumeConfig) error {
		vc.TypedSpec().Type = block.VolumeTypePartition
		vc.TypedSpec().Mount = block.MountSpec{
			TargetPath:   constants.StateMountPoint,
			SelinuxLabel: constants.StateSelinuxLabel,
			FileMode:     0o700,
			UID:          0,
			GID:          0,
		}

		match := labelVolumeMatchAndNonEmpty(constants.StatePartitionLabel)
		if ctrl.V1Alpha1Mode.IsAgent() { // mark as missing
			match = noMatch
		}

		// check here - make match false
		vc.TypedSpec().Locator = block.LocatorSpec{
			Match: match,
		}

		if encryptionMeta != nil {
			var encryptionFromMeta *v1alpha1.EncryptionConfig

			if err := json.Unmarshal([]byte(encryptionMeta.TypedSpec().Value), &encryptionFromMeta); err != nil {
				return fmt.Errorf("error unmarshalling state encryption meta key: %w", err)
			}

			if err := ctrl.convertEncryption(
				encryptionFromMeta,
				vc.TypedSpec(),
			); err != nil {
				return fmt.Errorf("error converting encryption for %s: %w", constants.StatePartitionLabel, err)
			}
		} else {
			vc.TypedSpec().Encryption = block.EncryptionSpec{}
		}

		return nil
	}
}

func (ctrl *VolumeConfigController) manageStandardVolumes(ctx context.Context, r controller.Runtime) error {
	if err := safe.WriterModify(ctx, r,
		block.NewVolumeConfig(block.NamespaceName, "/var/run"),
		func(vc *block.VolumeConfig) error {
			vc.TypedSpec().Type = block.VolumeTypeSymlink
			vc.TypedSpec().Symlink = block.SymlinkProvisioningSpec{
				SymlinkTargetPath: "/run",
				Force:             true,
			}
			vc.TypedSpec().Mount = block.MountSpec{
				TargetPath: "/var/run",
			}

			return nil
		},
	); err != nil {
		return fmt.Errorf("error creating symlink volume configuration for /var/run: %w", err)
	}

	parentIDs := map[string]string{
		"/var":     constants.EphemeralPartitionLabel,
		"/var/run": "/var/run",
	}

	for _, volume := range []struct {
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
	} {
		parentDir := filepath.Dir(volume.Path)
		targetDir := filepath.Base(volume.Path)

		parentID, ok := parentIDs[parentDir]
		if !ok {
			return fmt.Errorf("unknown parent directory volume %q for %q", parentDir, volume.Path)
		}

		volumeID := volume.Path

		if volume.ID != "" {
			volumeID = volume.ID
		}

		if err := safe.WriterModify(ctx, r,
			block.NewVolumeConfig(block.NamespaceName, volumeID),
			func(vc *block.VolumeConfig) error {
				vc.TypedSpec().Type = block.VolumeTypeDirectory

				vc.TypedSpec().Mount = block.MountSpec{
					TargetPath:       targetDir,
					ParentID:         parentID,
					SelinuxLabel:     volume.SELinuxLabel,
					FileMode:         volume.Mode,
					UID:              volume.UID,
					GID:              volume.GID,
					RecursiveRelabel: volume.Recursive,
				}

				return nil
			},
		); err != nil {
			return fmt.Errorf("error creating volume configuration for %q: %w", volume.Path, err)
		}

		parentIDs[volume.Path] = volumeID
	}

	return nil
}

func (ctrl *VolumeConfigController) manageOverlayVolumes(ctx context.Context, r controller.Runtime) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	for _, overlay := range constants.Overlays {
		if err := safe.WriterModify(ctx, r,
			block.NewVolumeConfig(block.NamespaceName, overlay.Path),
			func(vc *block.VolumeConfig) error {
				vc.TypedSpec().Type = block.VolumeTypeOverlay
				vc.TypedSpec().ParentID = constants.EphemeralPartitionLabel
				vc.TypedSpec().Mount = block.MountSpec{
					TargetPath:   overlay.Path,
					SelinuxLabel: overlay.Label,
					FileMode:     0o755,
					UID:          0,
					GID:          0,
				}

				return nil
			},
		); err != nil {
			return fmt.Errorf("error creating volume configuration for %q: %w", overlay.Path, err)
		}
	}

	return nil
}
