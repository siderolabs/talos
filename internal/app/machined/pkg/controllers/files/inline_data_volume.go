// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
)

// InlineDataVolumeController renders the files of inline data volumes.
type InlineDataVolumeController struct {
	// SkipFilesystemCheck disables the check that the volume is a tmpfs, for tests.
	SkipFilesystemCheck bool
}

// Name implements controller.Controller interface.
func (ctrl *InlineDataVolumeController) Name() string {
	return "files.InlineDataVolumeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *InlineDataVolumeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *InlineDataVolumeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.InlineDataVolumeStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *InlineDataVolumeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

type inlineDataVolume struct {
	// path opens the volume root, valid while the finalizer on the mount status is held.
	path   string
	device uint64
	hash   string
}

func (ctrl *InlineDataVolumeController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	inlineDataVolumeConfigs, err := configuredInlineDataVolumes(ctx, r)
	if err != nil {
		return err
	}

	volumeMountStatuses, err := safe.ReaderListAll[*block.VolumeMountStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list volume mount statuses: %w", err)
	}

	r.StartTrackingOutputs()

	released, errs := ctrl.renderVolumes(ctx, r, logger, inlineDataVolumeConfigs, volumeMountStatuses)

	if err = ctrl.releaseVolumes(ctx, r, logger, released); err != nil {
		errs = errors.Join(errs, err)
	}

	if err = safe.CleanupOutputs[*files.InlineDataVolumeStatus](ctx, r); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}

func configuredInlineDataVolumes(ctx context.Context, r controller.Reader) (map[string]configconfig.InlineDataVolumeConfig, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to get machine config: %w", err)
	}

	inlineDataVolumeConfigs := map[string]configconfig.InlineDataVolumeConfig{}

	if cfg != nil {
		for _, inlineDataVolumeConfig := range cfg.Config().InlineDataVolumeConfigs() {
			inlineDataVolumeConfigs[inlineDataVolumeConfig.Name()] = inlineDataVolumeConfig
		}
	}

	return inlineDataVolumeConfigs, nil
}

// renderVolumes renders the files of every configured inline data volume which is mounted.
//
// Returns the mounts of the volumes which are no longer configured or being unmounted, by name.
func (ctrl *InlineDataVolumeController) renderVolumes(
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	inlineDataVolumeConfigs map[string]configconfig.InlineDataVolumeConfig,
	volumeMountStatuses safe.List[*block.VolumeMountStatus],
) (map[string]*block.VolumeMountStatus, error) {
	var errs error

	released := map[string]*block.VolumeMountStatus{}

	for volumeMountStatus := range volumeMountStatuses.All() {
		name, ok := inlineDataVolumeName(volumeMountStatus)
		if !ok {
			continue
		}

		inlineDataVolumeConfig, configured := inlineDataVolumeConfigs[name]

		if !configured || volumeMountStatus.Metadata().Phase() != resource.PhaseRunning {
			released[name] = volumeMountStatus

			continue
		}

		if _, err := ctrl.renderVolume(ctx, r, logger, volumeMountStatus, inlineDataVolumeConfig); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to render inline data volume %q: %w", name, err))
		}
	}

	return released, errs
}

func inlineDataVolumeName(volumeMountStatus *block.VolumeMountStatus) (string, bool) {
	volumeID := volumeMountStatus.TypedSpec().VolumeID

	if volumeMountStatus.Metadata().ID() != volumeID {
		return "", false
	}

	return strings.CutPrefix(volumeID, constants.InlineDataVolumePrefix)
}

// renderVolume writes the files of an inline data volume through the file descriptor of its mount.
func (ctrl *InlineDataVolumeController) renderVolume(
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	volumeMountStatus *block.VolumeMountStatus, inlineDataVolumeConfig configconfig.InlineDataVolumeConfig,
) (inlineDataVolume, error) {
	if volumeMountStatus.TypedSpec().ReadOnly {
		return inlineDataVolume{}, errors.New("the volume is mounted read-only")
	}

	// keeps the file descriptor of the mount valid
	if !volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
			return inlineDataVolume{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	path, err := inlineDataVolumeRootPath(volumeMountStatus)
	if err != nil {
		return inlineDataVolume{}, err
	}

	root, err := os.OpenRoot(path)
	if err != nil {
		return inlineDataVolume{}, fmt.Errorf("failed to open volume root: %w", err)
	}

	defer root.Close() //nolint:errcheck

	device, err := ctrl.checkRoot(root)
	if err != nil {
		return inlineDataVolume{}, err
	}

	textFiles := inlineDataVolumeConfig.Files()

	hash, err := WriteInlineDataVolume(root, textFiles)
	if err != nil {
		return inlineDataVolume{}, err
	}

	if err = safe.WriterModify(ctx, r, files.NewInlineDataVolumeStatus(files.NamespaceName, inlineDataVolumeConfig.Name()),
		func(status *files.InlineDataVolumeStatus) error {
			if status.TypedSpec().ContentsHash != hash {
				logger.Info("rendered inline data volume",
					zap.String("volume", inlineDataVolumeConfig.Name()),
					zap.String("hash", hash),
					zap.Int("files", len(textFiles)),
				)
			}

			status.TypedSpec().VolumeID = volumeMountStatus.TypedSpec().VolumeID
			status.TypedSpec().ContentsHash = hash
			status.TypedSpec().Files = len(textFiles)
			status.TypedSpec().Device = device

			return nil
		},
	); err != nil {
		return inlineDataVolume{}, fmt.Errorf("failed to write status: %w", err)
	}

	return inlineDataVolume{path: path, device: device, hash: hash}, nil
}

// inlineDataVolumeRootPath returns the /proc/self/fd path of the detached mount.
func inlineDataVolumeRootPath(volumeMountStatus *block.VolumeMountStatus) (string, error) {
	root, ok := volumeMountStatus.TypedSpec().Root().(xfs.Root)
	if !ok || root == nil {
		return "", errors.New("the volume mount has no root")
	}

	fd, err := root.Fd()
	if err != nil {
		// in tests the mount is attached to a path
		if osRoot, isOSRoot := root.(*xfs.OSRoot); isOSRoot {
			return osRoot.Shadow, nil
		}

		return "", fmt.Errorf("failed to get the file descriptor of the volume mount: %w", err)
	}

	return fmt.Sprintf("/proc/self/fd/%d", fd), nil
}

// checkRoot returns the device number of the volume root.
func (ctrl *InlineDataVolumeController) checkRoot(root *os.Root) (uint64, error) {
	if !ctrl.SkipFilesystemCheck {
		if err := checkTmpfs(root); err != nil {
			return 0, err
		}

		// a tmpfs root is world-writable by default
		if err := root.Chmod(".", 0o755); err != nil {
			return 0, fmt.Errorf("failed to set mode of volume root: %w", err)
		}
	}

	info, err := root.Stat(".")
	if err != nil {
		return 0, fmt.Errorf("failed to stat volume root: %w", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("failed to stat volume root: no device number")
	}

	return stat.Dev, nil
}

// releaseVolumes releases the mounts of the inline data volumes which are no longer configured or being unmounted.
func (ctrl *InlineDataVolumeController) releaseVolumes(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, released map[string]*block.VolumeMountStatus,
) error {
	var errs error

	for name, volumeMountStatus := range released {
		if !volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to remove finalizer from the mount of inline data volume %q: %w", name, err))

			continue
		}

		logger.Info("released inline data volume", zap.String("volume", name))
	}

	return errs
}

func checkTmpfs(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("failed to open volume root: %w", err)
	}

	defer dir.Close() //nolint:errcheck

	var statfs unix.Statfs_t

	if err = unix.Fstatfs(int(dir.Fd()), &statfs); err != nil {
		return fmt.Errorf("failed to statfs volume root: %w", err)
	}

	if statfs.Type != unix.TMPFS_MAGIC {
		return fmt.Errorf("volume root is not a tmpfs (filesystem type %#x)", statfs.Type)
	}

	return nil
}
