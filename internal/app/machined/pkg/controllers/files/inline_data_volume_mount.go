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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
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

// errInvalidRequestID is a permanent failure, not retried.
var errInvalidRequestID = errors.New("request ID is not a valid file name")

// reconcileMounts serves InlineDataVolumeMountRequests.
//
// The binds of released volumes are torn down, so their consumers see them leave the running phase.
//
// Returns the names of volumes which still have binds, and whether any request failed.
// An error is returned only if listing fails.
func (ctrl *InlineDataVolumeController) reconcileMounts(
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	rendered map[string]inlineDataVolume, released map[string]*block.VolumeMountStatus,
) (map[string]struct{}, bool, error) {
	requests, err := safe.ReaderListAll[*files.InlineDataVolumeMountRequest](ctx, r)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list inline data volume mount requests: %w", err)
	}

	statuses, err := safe.ReaderListAll[*files.InlineDataVolumeMountStatus](ctx, r)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list inline data volume mount statuses: %w", err)
	}

	statusByID := map[resource.ID]*files.InlineDataVolumeMountStatus{}

	for status := range statuses.All() {
		statusByID[status.Metadata().ID()] = status
	}

	inUse := map[string]struct{}{}
	failures := mountFailures{logger: logger}

	requested := map[resource.ID]struct{}{}

	for request := range requests.All() {
		requestID := request.Metadata().ID()
		requested[requestID] = struct{}{}

		volumeName := request.TypedSpec().VolumeName
		_, isReleased := released[volumeName]

		bindReleased, msg, requestErr := ctrl.reconcileRequest(ctx, r, logger, request, statusByID[requestID], isReleased, rendered)
		failures.report(msg, requestID, requestErr)

		if !bindReleased {
			inUse[volumeName] = struct{}{}
		}
	}

	// the statuses of requests which are gone
	for status := range statuses.All() {
		statusID := status.Metadata().ID()

		if _, ok := requested[statusID]; ok {
			continue
		}

		bindReleased, releaseErr := ctrl.releaseMount(ctx, r, logger, statusID)
		failures.report("failed to release inline data volume bind", statusID, releaseErr)

		if !bindReleased {
			inUse[status.TypedSpec().VolumeName] = struct{}{}
		}
	}

	return inUse, failures.failed, nil
}

// reconcileRequest serves, unbinds or releases a single request.
//
// status is the current status of the request, nil if there is none, and isReleased whether its volume is released.
// Returns whether the bind is gone, and the message to report a failure with.
func (ctrl *InlineDataVolumeController) reconcileRequest(
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	request *files.InlineDataVolumeMountRequest, status *files.InlineDataVolumeMountStatus, isReleased bool,
	rendered map[string]inlineDataVolume,
) (bool, string, error) {
	switch {
	case request.Metadata().Phase() == resource.PhaseTearingDown:
		bindReleased, err := ctrl.releaseRequest(ctx, r, logger, request)

		return bindReleased, "failed to release inline data volume mount request", err
	case isReleased:
		const msg = "failed to release inline data volume bind"

		// a request without a bind holds nothing
		if status == nil {
			return true, msg, nil
		}

		bindReleased, err := ctrl.releaseMount(ctx, r, logger, request.Metadata().ID())

		return bindReleased, msg, err
	default:
		return true, "failed to serve inline data volume mount request", ctrl.serveRequest(ctx, r, logger, request, status, rendered)
	}
}

// mountFailures logs per-request errors and tracks whether any should be retried.
type mountFailures struct {
	logger *zap.Logger
	failed bool
}

func (f *mountFailures) report(msg, requestID string, err error) {
	if err == nil {
		return
	}

	f.logger.Error(msg, zap.String("request", requestID), zap.Error(err))

	if !errors.Is(err, errInvalidRequestID) {
		f.failed = true
	}
}

// releaseRequest releases the bind of a request being torn down, then the request itself.
//
// Reports whether the bind is gone.
func (ctrl *InlineDataVolumeController) releaseRequest(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, request *files.InlineDataVolumeMountRequest,
) (bool, error) {
	requestID := request.Metadata().ID()

	released, err := ctrl.releaseMount(ctx, r, logger, requestID)
	if !released {
		return false, err
	}

	if request.Metadata().Finalizers().Has(ctrl.Name()) {
		if removeErr := r.RemoveFinalizer(ctx, request.Metadata(), ctrl.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove finalizer from request %q: %w", requestID, removeErr))
		}
	}

	return true, err
}

// serveRequest binds the volume of a request once it is rendered.
//
// status is the current status of the request, nil if there is none.
func (ctrl *InlineDataVolumeController) serveRequest(
	ctx context.Context, r controller.Runtime, logger *zap.Logger,
	request *files.InlineDataVolumeMountRequest, status *files.InlineDataVolumeMountStatus, rendered map[string]inlineDataVolume,
) error {
	requestID := request.Metadata().ID()
	volumeName := request.TypedSpec().VolumeName

	if !request.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	volume, isRendered := rendered[volumeName]
	if !isRendered {
		return nil
	}

	target, err := ctrl.mountTarget(requestID)
	if err != nil {
		return err
	}

	if status != nil && status.Metadata().Phase() == resource.PhaseTearingDown {
		// left over from a previous request with the same ID
		released, releaseErr := ctrl.releaseMount(ctx, r, logger, requestID)
		if releaseErr != nil || !released {
			return releaseErr
		}
	}

	if err = ctrl.ensureBind(logger, requestID, volumeName, target, volume); err != nil {
		return fmt.Errorf("failed to bind inline data volume %q: %w", volumeName, err)
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

	switch {
	case errors.Is(err, errInvalidRequestID):
		// never bound
	case err != nil:
		return false, err
	default:
		if err = ctrl.binder().Unbind(target); err != nil {
			return false, fmt.Errorf("failed to remove the bind of %q: %w", requestID, err)
		}
	}

	if err = r.Destroy(ctx, statusMD); err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to destroy status %q: %w", requestID, err)
	}

	logger.Info("released inline data volume bind", zap.String("request", requestID), zap.String("target", target))

	return true, nil
}

func (ctrl *InlineDataVolumeController) mountTarget(requestID string) (string, error) {
	if requestID == "" || strings.HasPrefix(requestID, ".") || strings.ContainsAny(requestID, "/\x00") {
		return "", fmt.Errorf("%w: %q", errInvalidRequestID, requestID)
	}

	return filepath.Join(ctrl.mountPoint(), requestID), nil
}

func (ctrl *InlineDataVolumeController) mountPoint() string {
	if ctrl.MountPoint != "" {
		return ctrl.MountPoint
	}

	return constants.InlineDataVolumeMountPoint
}

func (ctrl *InlineDataVolumeController) binder() InlineDataVolumeBinder {
	if ctrl.Binder != nil {
		return ctrl.Binder
	}

	return MountBinder{}
}

// MountBinder is the default InlineDataVolumeBinder, which makes read-only bind mounts.
type MountBinder struct{}

// Bound implements InlineDataVolumeBinder.
func (MountBinder) Bound(target string, device uint64) (bool, error) {
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
func (b MountBinder) Bind(source, target string) error {
	if err := b.Unbind(target); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	if err := os.Mkdir(target, 0o700); err != nil {
		return fmt.Errorf("failed to create bind target: %w", err)
	}

	if err := mountv3.BindReadonly(source, target); err != nil {
		return errors.Join(err, os.Remove(target))
	}

	return nil
}

// Unbind implements InlineDataVolumeBinder.
func (MountBinder) Unbind(target string) error {
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
