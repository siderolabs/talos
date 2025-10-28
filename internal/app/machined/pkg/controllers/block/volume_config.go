// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	cfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Size constants.
const (
	MiB               = 1024 * 1024
	MinUserVolumeSize = 100 * MiB
)

// skipUserVolumeMountRequest is used to skip creating a VolumeMountRequest for a user volume.
type skipUserVolumeMountRequest struct{}

func defaultMountTransform[C cfg.NamedDocument](C, *block.VolumeMountRequest, string) error {
	return nil
}

func skipMountTransform[C cfg.NamedDocument](C, *block.VolumeMountRequest, string) error {
	return xerrors.NewTaggedf[skipUserVolumeMountRequest]("skip")
}

var noMatch = cel.MustExpression(cel.ParseBooleanExpression("false", celenv.Empty()))

// MetaProvider wraps acquiring meta.
type MetaProvider interface {
	Meta() machinedruntime.Meta
}

// VolumeConfigController provides volume configuration based on Talos defaults and machine configuration.
type VolumeConfigController struct {
	V1Alpha1Mode machinedruntime.Mode
	MetaProvider MetaProvider
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
func (ctrl *VolumeConfigController) Outputs() []controller.Output {
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

func convertEncryptionConfiguration(in cfg.EncryptionConfig, out *block.VolumeConfigSpec) error {
	if in == nil {
		out.Encryption = block.EncryptionSpec{}

		return nil
	}

	out.Encryption.Provider = in.Provider()
	out.Encryption.Cipher = in.Cipher()
	out.Encryption.KeySize = in.KeySize()
	out.Encryption.BlockSize = in.BlockSize()
	out.Encryption.PerfOptions = in.Options()

	out.Encryption.Keys = make([]block.EncryptionKey, len(in.Keys()))

	for i, key := range in.Keys() {
		out.Encryption.Keys[i].Slot = key.Slot()
		out.Encryption.Keys[i].LockToSTATE = key.LockToSTATE()

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
//nolint:gocyclo,cyclop
func (ctrl *VolumeConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		// load config if present
		machineCfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
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
		configurationPresent := machineCfg != nil && machineCfg.Config().Machine() != nil

		// STATE configuration should be always created, but it depends on the configuration presence
		if configurationPresent {
			err = safe.WriterModify(ctx, r,
				block.NewVolumeConfig(block.NamespaceName, constants.StatePartitionLabel),
				ctrl.manageStateConfigPresent(ctx, logger, machineCfg.Config()),
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
				ctrl.manageEphemeral(machineCfg.Config()),
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

		// fetch all user-defined volume configs
		var (
			userVolumeConfigs     []cfg.UserVolumeConfig
			rawVolumeConfigs      []cfg.RawVolumeConfig
			existingVolumeConfigs []cfg.ExistingVolumeConfig
			swapVolumeConfigs     []cfg.SwapVolumeConfig
		)

		if machineCfg != nil {
			userVolumeConfigs = machineCfg.Config().UserVolumeConfigs()
			rawVolumeConfigs = machineCfg.Config().RawVolumeConfigs()
			existingVolumeConfigs = machineCfg.Config().ExistingVolumeConfigs()
			swapVolumeConfigs = machineCfg.Config().SwapVolumeConfigs()
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
				ctrl.handleExistingVolumeMountRequest,
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

		encryptionConfig := extraVolumeConfig.Encryption()
		if encryptionConfig == nil {
			// fall back to v1alpha1 encryption config
			encryptionConfig = config.Machine().SystemDiskEncryption().Get(constants.EphemeralPartitionLabel)
		}

		if err := convertEncryptionConfiguration(
			encryptionConfig,
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

func (ctrl *VolumeConfigController) manageStateConfigPresent(ctx context.Context, logger *zap.Logger, config cfg.Config) func(vc *block.VolumeConfig) error {
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
				MinSize:  quirks.New("").PartitionSizes().StateSize(),
				MaxSize:  quirks.New("").PartitionSizes().StateSize(),
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

		extraVolumeConfig, _ := config.Volumes().ByName(constants.StatePartitionLabel)

		encryptionConfig := extraVolumeConfig.Encryption()
		if encryptionConfig == nil {
			// fall back to v1alpha1 encryption config
			encryptionConfig = config.Machine().SystemDiskEncryption().Get(constants.StatePartitionLabel)
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

		logger.Info("saved state encryption config to META")

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
			encryptionFromMeta, err := UnmarshalEncryptionMeta([]byte(encryptionMeta.TypedSpec().Value))
			if err != nil {
				return err
			}

			if err := convertEncryptionConfiguration(
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

func (ctrl *VolumeConfigController) handleUserVolumeConfig(
	userVolumeConfig cfg.UserVolumeConfig,
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

//nolint:dupl
func (ctrl *VolumeConfigController) handleRawVolumeConfig(
	rawVolumeConfig cfg.RawVolumeConfig,
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

func (ctrl *VolumeConfigController) handleExistingVolumeConfig(
	existingVolumeConfig cfg.ExistingVolumeConfig,
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

func (ctrl *VolumeConfigController) handleExistingVolumeMountRequest(
	existingVolumeConfig cfg.ExistingVolumeConfig,
	m *block.VolumeMountRequest,
	_ string,
) error {
	m.TypedSpec().ReadOnly = existingVolumeConfig.Mount().ReadOnly()

	return nil
}

//nolint:dupl
func (ctrl *VolumeConfigController) handleSwapVolumeConfig(
	swapVolumeConfig cfg.SwapVolumeConfig,
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

// handleCustomVolumeConfig handled transormation of a custom (user) volume configuration
// into VolumeConfig and VolumeMountRequest resources.
//
// The function is generic accepting some common properties:
// - prefix is used to create the volume ID from the config document name
// - label is used to set the label on the VolumeConfig/VolumeMountRequest
// - transformFunc is a function that transforms the config document into VolumeConfig spec.
func handleCustomVolumeConfig[C cfg.NamedDocument](
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
