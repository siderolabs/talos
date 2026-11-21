// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// InlineDataVolumeBinder makes the read-only binds of inline data volumes.
type InlineDataVolumeBinder interface {
	// Bound reports whether target is a bind of the filesystem with the device number.
	Bound(target string, device uint64) (bool, error)
	// Bind binds the root of the volume opened by source at target, read-only.
	Bind(source, target string) error
	// Unbind removes the bind at target, if any, and the target itself.
	Unbind(target string) error
}

// reconcileMounts serves InlineDataVolumeMountRequests, returning the names of volumes which still have binds.
func (ctrl *InlineDataVolumeController) reconcileMounts(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, rendered map[string]inlineDataVolume,
) (map[string]struct{}, error) {
	inUse := map[string]struct{}{}

	requests, err := safe.ReaderListAll[*files.InlineDataVolumeMountRequest](ctx, r)
	if err != nil {
		return inUse, fmt.Errorf("failed to list inline data volume mount requests: %w", err)
	}

	statuses, err := safe.ReaderListAll[*files.InlineDataVolumeMountStatus](ctx, r)
	if err != nil {
		return inUse, fmt.Errorf("failed to list inline data volume mount statuses: %w", err)
	}

	var errs error

	requested := map[resource.ID]struct{}{}

	for request := range requests.All() {
		requested[request.Metadata().ID()] = struct{}{}

		if request.Metadata().Phase() == resource.PhaseTearingDown {
			held, releaseErr := ctrl.releaseRequest(ctx, r, logger, request)
			errs = errors.Join(errs, releaseErr)

			if held {
				inUse[request.TypedSpec().VolumeName] = struct{}{}
			}

			continue
		}

		inUse[request.TypedSpec().VolumeName] = struct{}{}

		errs = errors.Join(errs, ctrl.serveRequest(ctx, r, logger, request, rendered))
	}

	// the statuses of requests which are gone
	for status := range statuses.All() {
		if _, ok := requested[status.Metadata().ID()]; ok {
			continue
		}

		released, releaseErr := ctrl.releaseMount(ctx, r, logger, status.Metadata().ID())
		errs = errors.Join(errs, releaseErr)

		if !released {
			inUse[status.TypedSpec().VolumeName] = struct{}{}
		}
	}

	return inUse, errs
}

func (ctrl *InlineDataVolumeController) serveRequest(
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	request *files.InlineDataVolumeMountRequest, rendered map[string]inlineDataVolume,
) error {
	requestID := request.Metadata().ID()
	volumeName := request.TypedSpec().VolumeName

	if !request.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to add finalizer to request %q: %w", requestID, err)
		}
	}

	volume, isRendered := rendered[volumeName]
	if !isRendered {
		return nil
	}

	if err := ctrl.reconcileMount(ctx, r, logger, requestID, volumeName, volume); err != nil {
		return fmt.Errorf("failed to bind inline data volume %q for %q: %w", volumeName, requestID, err)
	}

	return nil
}

// releaseRequest reports whether the volume is still held by the bind.
func (ctrl *InlineDataVolumeController) releaseRequest(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, request *files.InlineDataVolumeMountRequest,
) (bool, error) {
	requestID := request.Metadata().ID()

	released, err := ctrl.releaseMount(ctx, r, logger, requestID)
	if !released {
		return true, err
	}

	if request.Metadata().Finalizers().Has(ctrl.Name()) {
		if removeErr := r.RemoveFinalizer(ctx, request.Metadata(), ctrl.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove finalizer from request %q: %w", requestID, removeErr))
		}
	}

	return false, err
}

func (ctrl *InlineDataVolumeController) reconcileMount(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, requestID, volumeName string, volume inlineDataVolume,
) error {
	target, err := ctrl.mountTarget(requestID)
	if err != nil {
		return err
	}

	status, err := safe.ReaderGetByID[*files.InlineDataVolumeMountStatus](ctx, r, requestID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if status != nil && status.Metadata().Phase() == resource.PhaseTearingDown {
		// left over from a previous request with the same ID
		_, err = ctrl.releaseMount(ctx, r, logger, requestID)

		return err
	}

	if err = ctrl.ensureBind(logger, requestID, volumeName, target, volume); err != nil {
		return err
	}

	return safe.WriterModify(ctx, r, files.NewInlineDataVolumeMountStatus(files.NamespaceName, requestID),
		func(status *files.InlineDataVolumeMountStatus) error {
			status.TypedSpec().VolumeName = volumeName
			status.TypedSpec().Target = target
			status.TypedSpec().ContentsHash = volume.hash
			status.TypedSpec().Device = volume.device

			return nil
		},
	)
}

func (ctrl *InlineDataVolumeController) ensureBind(logger *zap.Logger, requestID, volumeName, target string, volume inlineDataVolume) error {
	bound, err := ctrl.binder().Bound(target, volume.device)
	if err != nil || bound {
		return err
	}

	if err = ctrl.binder().Bind(volume.path, target); err != nil {
		return err
	}

	logger.Info("bound inline data volume read-only",
		zap.String("volume", volumeName), zap.String("request", requestID), zap.String("target", target))

	return nil
}

// releaseMount reports whether the bind is gone.
func (ctrl *InlineDataVolumeController) releaseMount(ctx context.Context, r controller.Runtime, logger *zap.Logger, requestID string) (bool, error) {
	statusMD := files.NewInlineDataVolumeMountStatus(files.NamespaceName, requestID).Metadata()

	okToDestroy, err := r.Teardown(ctx, statusMD)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to tear down status %q: %w", requestID, err)
	}

	if err == nil && !okToDestroy {
		return false, nil
	}

	target, err := ctrl.mountTarget(requestID)
	if err != nil {
		return false, err
	}

	if err = ctrl.binder().Unbind(target); err != nil {
		return false, fmt.Errorf("failed to remove the bind of %q: %w", requestID, err)
	}

	if err = r.Destroy(ctx, statusMD); err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to destroy status %q: %w", requestID, err)
	}

	logger.Info("released inline data volume bind", zap.String("request", requestID), zap.String("target", target))

	return true, nil
}

func (ctrl *InlineDataVolumeController) mountTarget(requestID string) (string, error) {
	if requestID == "" || strings.HasPrefix(requestID, ".") || strings.ContainsAny(requestID, "/\x00") {
		return "", fmt.Errorf("request ID %q is not a valid file name", requestID)
	}

	return filepath.Join(ctrl.mountPoint(), requestID), nil
}

func (ctrl *InlineDataVolumeController) mountPoint() string {
	if ctrl.MountPoint != "" {
		return ctrl.MountPoint
	}

	return defaultInlineDataVolumeMountPoint
}

func (ctrl *InlineDataVolumeController) binder() InlineDataVolumeBinder {
	if ctrl.Binder != nil {
		return ctrl.Binder
	}

	return mountBinder{}
}

type mountBinder struct{}

// Bound implements InlineDataVolumeBinder.
func (mountBinder) Bound(target string, device uint64) (bool, error) {
	var targetStat, parentStat unix.Stat_t

	if err := unix.Lstat(target, &targetStat); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("failed to stat %q: %w", target, err)
	}

	if err := unix.Lstat(filepath.Dir(target), &parentStat); err != nil {
		return false, fmt.Errorf("failed to stat %q: %w", filepath.Dir(target), err)
	}

	return targetStat.Mode&unix.S_IFMT == unix.S_IFDIR && targetStat.Dev == device && parentStat.Dev != targetStat.Dev, nil
}

// Bind implements InlineDataVolumeBinder.
func (b mountBinder) Bind(source, target string) error {
	if err := b.Unbind(target); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	if err := os.Mkdir(target, 0o700); err != nil {
		return fmt.Errorf("failed to create bind target: %w", err)
	}

	sourceFD, err := unix.Open(source, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("failed to open volume root: %w", err)
	}

	defer unix.Close(sourceFD) //nolint:errcheck

	if err = mountv3.BindReadonlyFd(sourceFD, target); err != nil {
		return errors.Join(err, os.Remove(target))
	}

	return nil
}

// Unbind implements InlineDataVolumeBinder.
func (mountBinder) Unbind(target string) error {
	// a mount may be stacked on top of another one
	for {
		err := unix.Unmount(target, unix.MNT_DETACH)
		if err == nil {
			continue
		}

		if errors.Is(err, syscall.EINVAL) || errors.Is(err, fs.ErrNotExist) {
			break
		}

		return fmt.Errorf("failed to unmount %q: %w", target, err)
	}

	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove %q: %w", target, err)
	}

	return nil
}
