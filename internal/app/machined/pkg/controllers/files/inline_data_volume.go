// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

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

// inlineDataVolumeRetryInterval is the delay before retrying to render the inline data volumes which failed.
const inlineDataVolumeRetryInterval = 30 * time.Second

// InlineDataVolumeController renders the files of inline data volumes and binds them read-only.
//
// A failed render or bind is logged and retried without restarting the controller, so other volumes aren't held back.
type InlineDataVolumeController struct {
	// Binder makes the read-only binds, defaults to bind mounts.
	Binder InlineDataVolumeBinder
	// MountPoint is the directory of the binds, defaults to constants.InlineDataVolumeMountPoint.
	MountPoint string
	// SkipFilesystemCheck disables the check that the volume is a tmpfs, and allows volume mounts
	// attached to a path instead of a detached mount file descriptor, for tests only.
	SkipFilesystemCheck bool

	// HashKey keys the contents hash; random if unset.
	HashKey []byte
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
		{
			Namespace: files.NamespaceName,
			Type:      files.InlineDataVolumeMountRequestType,
			Kind:      controller.InputStrong,
		},
		{
			// the consumers remove their finalizers from the statuses
			Namespace: files.NamespaceName,
			Type:      files.InlineDataVolumeMountStatusType,
			Kind:      controller.InputDestroyReady,
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
		{
			Type: files.InlineDataVolumeMountStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *InlineDataVolumeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.HashKey == nil {
		ctrl.HashKey = make([]byte, 32)

		if _, err := rand.Read(ctrl.HashKey); err != nil {
			return fmt.Errorf("failed to generate hash key: %w", err)
		}
	}

	retryTimer := time.NewTimer(inlineDataVolumeRetryInterval)
	retryTimer.Stop()

	defer retryTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-retryTimer.C:
		}

		failed, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			return err
		}

		if failed {
			retryTimer.Reset(inlineDataVolumeRetryInterval)
		} else {
			retryTimer.Stop()
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

// reconcile renders and releases the inline data volumes.
//
// Returns whether any volume failed to render or bind, and an error only for failures of the whole controller.
func (ctrl *InlineDataVolumeController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (bool, error) {
	inlineDataVolumeConfigs, err := configuredInlineDataVolumes(ctx, r)
	if err != nil {
		return false, err
	}

	volumeMountStatuses, err := safe.ReaderListAll[*block.VolumeMountStatus](ctx, r)
	if err != nil {
		return false, fmt.Errorf("failed to list volume mount statuses: %w", err)
	}

	r.StartTrackingOutputs()

	rendered, released, failed, errs := ctrl.renderVolumes(ctx, r, logger, inlineDataVolumeConfigs, volumeMountStatuses)

	inUse, mountsFailed, err := ctrl.reconcileMounts(ctx, r, logger, rendered, released)
	if err != nil {
		// volumes in use unknown, don't release any
		errs = errors.Join(errs, err)
	} else {
		// the binds of released volumes are being torn down; the ones still held (by a running
		// container) pin the filesystem, so the volume mount is kept until they are gone
		for name := range inUse {
			delete(released, name)
		}

		if err = ctrl.releaseVolumes(ctx, r, logger, released); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if err = safe.CleanupOutputs[*files.InlineDataVolumeStatus](ctx, r); err != nil {
		errs = errors.Join(errs, err)
	}

	return failed || mountsFailed, errs
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

// renderVolumes renders every configured, mounted inline data volume.
//
// Returns the rendered volumes, the mounts to release, and whether any render failed.
// A failed volume keeps its previous files and status.
func (ctrl *InlineDataVolumeController) renderVolumes(
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	inlineDataVolumeConfigs map[string]configconfig.InlineDataVolumeConfig,
	volumeMountStatuses safe.List[*block.VolumeMountStatus],
) (map[string]inlineDataVolume, map[string]*block.VolumeMountStatus, bool, error) {
	var (
		errs   error
		failed bool
	)

	rendered := map[string]inlineDataVolume{}
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

		volume, err := ctrl.renderVolume(ctx, r, logger, volumeMountStatus, inlineDataVolumeConfig)
		if err != nil {
			failed = true

			logger.Error("failed to render inline data volume", zap.String("volume", name), zap.Error(err))

			if err = keepStatus(ctx, r, name); err != nil {
				errs = errors.Join(errs, err)
			}

			continue
		}

		rendered[name] = volume
	}

	return rendered, released, failed, errs
}

// keepStatus keeps the existing status of an inline data volume through the cleanup of the outputs.
func keepStatus(ctx context.Context, r controller.Runtime, name string) error {
	_, err := safe.ReaderGetByID[*files.InlineDataVolumeStatus](ctx, r, name)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get status of inline data volume %q: %w", name, err)
	}

	if err = safe.WriterModify(ctx, r, files.NewInlineDataVolumeStatus(files.NamespaceName, name),
		func(*files.InlineDataVolumeStatus) error { return nil },
	); err != nil {
		return fmt.Errorf("failed to keep status of inline data volume %q: %w", name, err)
	}

	return nil
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

	path, err := ctrl.RootPath(volumeMountStatus)
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

	hash, err := WriteInlineDataVolume(root, ctrl.HashKey, textFiles)
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

// RootPath returns the /proc/self/fd path of the detached mount.
//
// A mount without a file descriptor is refused: the path it is attached to might not be the volume
// (e.g. a host directory when the mount status was rebuilt without the active mount).
func (ctrl *InlineDataVolumeController) RootPath(volumeMountStatus *block.VolumeMountStatus) (string, error) {
	root, ok := volumeMountStatus.TypedSpec().Root().(xfs.Root)
	if !ok || root == nil {
		return "", errors.New("the volume mount has no root")
	}

	fd, err := root.Fd()
	if err != nil {
		// in tests the mount is attached to a path
		if osRoot, isOSRoot := root.(*xfs.OSRoot); isOSRoot && ctrl.SkipFilesystemCheck {
			return osRoot.Shadow, nil
		}

		return "", fmt.Errorf("the volume mount has no file descriptor of a detached mount: %w", err)
	}

	return fmt.Sprintf("/proc/self/fd/%d", fd), nil
}

// checkRoot returns the device number of the volume root.
func (ctrl *InlineDataVolumeController) checkRoot(root *os.Root) (uint64, error) {
	if !ctrl.SkipFilesystemCheck {
		if err := CheckTmpfs(root); err != nil {
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

// releaseVolumes removes the finalizer from the mounts of released volumes.
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

// CheckTmpfs returns an error if the root is not a tmpfs.
func CheckTmpfs(root *os.Root) error {
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
