// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/swap"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/filetree"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type mountContext struct {
	point     *mount.Point
	readOnly  bool
	unmounter func() error
}

// MountController performs actual mount/unmount operations based on the MountRequests.
type MountController struct {
	activeMounts map[string]*mountContext
}

// Name implements controller.Controller interface.
func (ctrl *MountController) Name() string {
	return "block.MountController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MountController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.MountRequestType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.MountStatusType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MountController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.MountStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *MountController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.activeMounts == nil {
		ctrl.activeMounts = map[string]*mountContext{}
	}

	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read volume statuses: %w", err)
		}

		volumeStatusMap := xslices.ToMap(
			safe.ToSlice(
				volumeStatuses,
				identity,
			),
			func(v *block.VolumeStatus) (string, *block.VolumeStatus) {
				return v.Metadata().ID(), v
			},
		)

		mountStatuses, err := safe.ReaderListAll[*block.MountStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read mount statuses: %w", err)
		}

		mountStatusMap := xslices.ToMap(
			safe.ToSlice(
				mountStatuses,
				identity,
			),
			func(v *block.MountStatus) (string, *block.MountStatus) {
				return v.Metadata().ID(), v
			},
		)

		mountRequests, err := safe.ReaderListAll[*block.MountRequest](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to read mount requests: %w", err)
		}

		for mountRequest := range mountRequests.All() {
			volumeStatus := volumeStatusMap[mountRequest.TypedSpec().VolumeID]
			volumeNotReady := volumeStatus == nil || volumeStatus.TypedSpec().Phase != block.VolumePhaseReady || volumeStatus.Metadata().Phase() != resource.PhaseRunning

			mountRequestTearingDown := mountRequest.Metadata().Phase() == resource.PhaseTearingDown

			mountStatus := mountStatusMap[mountRequest.Metadata().ID()]
			mountStatusTearingDown := mountStatus != nil && mountStatus.Metadata().Phase() == resource.PhaseTearingDown

			mountHasParent := mountRequest.TypedSpec().ParentMountID != ""
			mountParentStatus := mountStatusMap[mountRequest.TypedSpec().ParentMountID] // this might be nil
			mountParentReady := !mountHasParent || (mountParentStatus != nil && mountParentStatus.Metadata().Phase() == resource.PhaseRunning)
			mountParentTearingDown := mountHasParent && mountParentStatus != nil && mountParentStatus.Metadata().Phase() == resource.PhaseTearingDown

			parentFinalizerName := ctrl.Name() + "-" + mountRequest.Metadata().ID()

			if volumeNotReady || mountRequestTearingDown || mountStatusTearingDown || mountParentTearingDown {
				// we should tear down the mount in the following sequence:
				// 1. tear down & destroy MountStatus
				// 2. perform actual unmount
				// 3. remove finalizer from VolumeStatus
				// 4. remove finalizer from parent MountStatus (if any)
				// 5. remove finalizer from MountRequest
				mountStatusTornDown, err := ctrl.tearDownMountStatus(ctx, r, logger, mountRequest)
				if err != nil {
					return fmt.Errorf("error tearing down mount status %q: %w", mountRequest.Metadata().ID(), err)
				}

				if !mountStatusTornDown {
					continue
				}

				if volumeStatus != nil {
					if err = ctrl.handleUnmountOperation(logger, mountRequest, volumeStatus); err != nil {
						return err
					}
				}

				if volumeStatus != nil && volumeStatus.Metadata().Finalizers().Has(ctrl.Name()) {
					if err = r.RemoveFinalizer(ctx, volumeStatus.Metadata(), ctrl.Name()); err != nil {
						return fmt.Errorf("failed to remove finalizer from volume status %q: %w", volumeStatus.Metadata().ID(), err)
					}
				}

				if mountParentStatus != nil && mountParentStatus.Metadata().Finalizers().Has(parentFinalizerName) {
					if err = r.RemoveFinalizer(ctx, mountParentStatus.Metadata(), parentFinalizerName); err != nil {
						return fmt.Errorf("failed to remove finalizer from parent mount status %q: %w", mountParentStatus.Metadata().ID(), err)
					}
				}

				if mountRequest.Metadata().Finalizers().Has(ctrl.Name()) {
					if err = r.RemoveFinalizer(ctx, mountRequest.Metadata(), ctrl.Name()); err != nil {
						return fmt.Errorf("failed to remove finalizer from mount request %q: %w", mountRequest.Metadata().ID(), err)
					}
				}
			}

			if !(volumeNotReady || mountRequestTearingDown) && mountParentReady {
				// we should perform mount operation in the following sequence:
				// 1. add finalizer on MountRequest
				// 2. add finalizer on parent MountStatus (if any)
				// 3. add finalizer on VolumeStatus
				// 4. perform actual mount
				// 5. create MountStatus
				if !mountRequest.Metadata().Finalizers().Has(ctrl.Name()) {
					if err = r.AddFinalizer(ctx, mountRequest.Metadata(), ctrl.Name()); err != nil {
						return fmt.Errorf("failed to add finalizer to mount request %q: %w", mountRequest.Metadata().ID(), err)
					}
				}

				if mountHasParent && !mountParentStatus.Metadata().Finalizers().Has(parentFinalizerName) {
					if err = r.AddFinalizer(ctx, mountParentStatus.Metadata(), parentFinalizerName); err != nil {
						return fmt.Errorf("failed to add finalizer to parent mount status %q: %w", mountParentStatus.Metadata().ID(), err)
					}
				}

				if !volumeStatus.Metadata().Finalizers().Has(ctrl.Name()) {
					if err = r.AddFinalizer(ctx, volumeStatus.Metadata(), ctrl.Name()); err != nil {
						return fmt.Errorf("failed to add finalizer to volume status %q: %w", volumeStatus.Metadata().ID(), err)
					}
				}

				mountSource := volumeStatus.TypedSpec().MountLocation
				mountTarget := volumeStatus.TypedSpec().MountSpec.TargetPath
				mountFilesystem := volumeStatus.TypedSpec().Filesystem

				rootPath := "/"

				if mountHasParent {
					rootPath = mountParentStatus.TypedSpec().Target
				}

				if err = ctrl.handleMountOperation(logger, rootPath, mountSource, mountTarget, mountFilesystem, mountRequest, volumeStatus); err != nil {
					return err
				}

				if err = safe.WriterModify(
					ctx, r, block.NewMountStatus(block.NamespaceName, mountRequest.Metadata().ID()),
					func(mountStatus *block.MountStatus) error {
						mountStatus.TypedSpec().Spec = *mountRequest.TypedSpec()
						mountStatus.TypedSpec().Source = mountSource
						mountStatus.TypedSpec().Target = filepath.Join(rootPath, mountTarget)
						mountStatus.TypedSpec().Filesystem = mountFilesystem
						mountStatus.TypedSpec().EncryptionProvider = volumeStatus.TypedSpec().EncryptionProvider
						mountStatus.TypedSpec().ReadOnly = mountRequest.TypedSpec().ReadOnly
						mountStatus.TypedSpec().ProjectQuotaSupport = volumeStatus.TypedSpec().MountSpec.ProjectQuotaSupport

						return nil
					},
				); err != nil {
					return fmt.Errorf("failed to create mount status %q: %w", mountRequest.Metadata().ID(), err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *MountController) tearDownMountStatus(ctx context.Context, r controller.Runtime, logger *zap.Logger, mountRequest *block.MountRequest) (bool, error) {
	logger = logger.With(zap.String("mount_request", mountRequest.Metadata().ID()))

	okToDestroy, err := r.Teardown(ctx, block.NewMountStatus(block.NamespaceName, mountRequest.Metadata().ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			// no mount status, we are done
			return true, nil
		}

		return false, fmt.Errorf("failed to teardown mount status %q: %w", mountRequest.Metadata().ID(), err)
	}

	if !okToDestroy {
		logger.Debug("waiting for mount status to be torn down")

		return false, nil
	}

	err = r.Destroy(ctx, block.NewMountStatus(block.NamespaceName, mountRequest.Metadata().ID()).Metadata())
	if err != nil {
		return false, fmt.Errorf("failed to destroy mount status %q: %w", mountRequest.Metadata().ID(), err)
	}

	return true, nil
}

func (ctrl *MountController) handleMountOperation(
	logger *zap.Logger,
	rootPath string,
	mountSource, mountTarget string,
	mountFilesystem block.FilesystemType,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	switch volumeStatus.TypedSpec().Type {
	case block.VolumeTypeDirectory:
		return ctrl.handleDirectoryMountOperation(rootPath, mountTarget, volumeStatus)
	case block.VolumeTypeOverlay:
		return ctrl.handleOverlayMountOperation(logger, filepath.Join(rootPath, mountTarget), mountRequest, volumeStatus)
	case block.VolumeTypeSymlink:
		return ctrl.handleSymlinkMountOperation(logger, rootPath, mountTarget, mountRequest, volumeStatus)
	case block.VolumeTypeTmpfs:
		return fmt.Errorf("not implemented yet")
	case block.VolumeTypeDisk, block.VolumeTypePartition:
		if mountFilesystem == block.FilesystemTypeSwap {
			return ctrl.handleSwapMountOperation(logger, mountSource, mountRequest, volumeStatus)
		}

		return ctrl.handleDiskMountOperation(logger, mountSource, filepath.Join(rootPath, mountTarget), mountFilesystem, mountRequest, volumeStatus)
	default:
		return fmt.Errorf("unsupported volume type %q", volumeStatus.TypedSpec().Type)
	}
}

func (ctrl *MountController) handleDirectoryMountOperation(
	rootPath string,
	target string,
	volumeStatus *block.VolumeStatus,
) error {
	targetPath := filepath.Join(rootPath, target)

	if err := os.Mkdir(targetPath, volumeStatus.TypedSpec().MountSpec.FileMode); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create target path: %w", err)
		}

		st, err := os.Stat(targetPath)
		if err != nil {
			return fmt.Errorf("failed to stat target path: %w", err)
		}

		if !st.IsDir() {
			return fmt.Errorf("target path %q is not a directory", targetPath)
		}
	}

	return ctrl.updateTargetSettings(targetPath, volumeStatus.TypedSpec().MountSpec)
}

//nolint:gocyclo
func (ctrl *MountController) handleSymlinkMountOperation(
	logger *zap.Logger,
	rootPath string,
	target string,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	_, ok := ctrl.activeMounts[mountRequest.Metadata().ID()]
	if ok {
		return nil
	}

	targetPath := filepath.Join(rootPath, target)

	st, err := os.Lstat(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat target path: %w", err)
	}

	if st == nil {
		// create the symlink
		if err := os.Symlink(volumeStatus.TypedSpec().SymlinkSpec.SymlinkTargetPath, targetPath); err != nil {
			return fmt.Errorf("failed to create symlink %q: %w", targetPath, err)
		}

		ctrl.activeMounts[mountRequest.Metadata().ID()] = &mountContext{}

		return nil
	}

	if st.Mode()&os.ModeSymlink != 0 {
		// if it's already a symlink, check if it points to the right target
		symlinkTarget, err := os.Readlink(targetPath)
		if err != nil {
			return fmt.Errorf("failed to read symlink target: %w", err)
		}

		if symlinkTarget == volumeStatus.TypedSpec().SymlinkSpec.SymlinkTargetPath {
			return nil
		}
	}

	if !volumeStatus.TypedSpec().SymlinkSpec.Force {
		return fmt.Errorf("target path %q is not a symlink to %q", targetPath, volumeStatus.TypedSpec().SymlinkSpec.SymlinkTargetPath)
	}

	// try to remove forcefully
	if err := os.RemoveAll(targetPath); err != nil {
		if !st.Mode().IsDir() {
			return fmt.Errorf("failed to remove target path, and target is not a directory %s: %w", st.Mode(), err)
		}

		// try to remove all entries if it's a directory
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return fmt.Errorf("failed to read target path: %w", err)
		}

		for _, entry := range entries {
			if err := os.RemoveAll(filepath.Join(targetPath, entry.Name())); err != nil {
				logger.Warn("failed to remove target path entry", zap.String("entry", entry.Name()), zap.Error(err))
			}
		}

		ctrl.activeMounts[mountRequest.Metadata().ID()] = &mountContext{}

		// return early, i.e. keep this as a directory
		return nil
	}

	if err := os.Symlink(volumeStatus.TypedSpec().SymlinkSpec.SymlinkTargetPath, targetPath); err != nil {
		return fmt.Errorf("failed to create symlink %q: %w", targetPath, err)
	}

	ctrl.activeMounts[mountRequest.Metadata().ID()] = &mountContext{}

	return nil
}

//nolint:gocyclo
func (ctrl *MountController) updateTargetSettings(
	targetPath string,
	mountSpec block.MountSpec,
) error {
	if err := os.Chmod(targetPath, mountSpec.FileMode); err != nil {
		return fmt.Errorf("failed to chmod %q: %w", targetPath, err)
	}

	st, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("failed to stat %q: %w", targetPath, err)
	}

	sysStat := st.Sys().(*syscall.Stat_t)

	if sysStat.Uid != uint32(mountSpec.UID) || sysStat.Gid != uint32(mountSpec.GID) {
		if mountSpec.RecursiveRelabel {
			err = filetree.ChownRecursive(targetPath, uint32(mountSpec.UID), uint32(mountSpec.GID))
		} else {
			err = os.Chown(targetPath, mountSpec.UID, mountSpec.GID)
		}

		if err != nil {
			return fmt.Errorf("failed to chown %q: %w", targetPath, err)
		}
	}

	currentLabel, err := selinux.GetLabel(targetPath)
	if err != nil {
		return fmt.Errorf("failed to get current label %q: %w", targetPath, err)
	}

	if currentLabel == mountSpec.SelinuxLabel {
		// nothing to do
		return nil
	}

	if mountSpec.RecursiveRelabel {
		err = selinux.SetLabelRecursive(targetPath, mountSpec.SelinuxLabel)
	} else {
		err = selinux.SetLabel(targetPath, mountSpec.SelinuxLabel)
	}

	if err != nil {
		return fmt.Errorf("error setting label %q: %w", targetPath, err)
	}

	return nil
}

func (ctrl *MountController) handleDiskMountOperation(
	logger *zap.Logger,
	mountSource, mountTarget string,
	mountFilesystem block.FilesystemType,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	mountCtx, ok := ctrl.activeMounts[mountRequest.Metadata().ID()]

	// mount hasn't been done yet
	if !ok {
		var opts []mount.NewPointOption

		opts = append(opts,
			mount.WithProjectQuota(volumeStatus.TypedSpec().MountSpec.ProjectQuotaSupport),
			mount.WithSelinuxLabel(volumeStatus.TypedSpec().MountSpec.SelinuxLabel),
		)

		if mountRequest.TypedSpec().ReadOnly {
			opts = append(opts, mount.WithReadonly())
		}

		mountpoint := mount.NewPoint(
			mountSource,
			mountTarget,
			mountFilesystem.String(),
			opts...,
		)

		unmounter, err := mountpoint.Mount(mount.WithMountPrinter(logger.Sugar().Infof))
		if err != nil {
			return fmt.Errorf("failed to mount %q: %w", mountRequest.Metadata().ID(), err)
		}

		if !mountRequest.TypedSpec().ReadOnly {
			if err = ctrl.updateTargetSettings(mountTarget, volumeStatus.TypedSpec().MountSpec); err != nil {
				unmounter() //nolint:errcheck

				return fmt.Errorf("failed to update target settings %q: %w", mountRequest.Metadata().ID(), err)
			}
		}

		logger.Info("volume mount",
			zap.String("volume", volumeStatus.Metadata().ID()),
			zap.String("source", mountSource),
			zap.String("target", mountTarget),
			zap.Stringer("filesystem", mountFilesystem),
			zap.Bool("read_only", mountRequest.TypedSpec().ReadOnly),
		)

		ctrl.activeMounts[mountRequest.Metadata().ID()] = &mountContext{
			point:     mountpoint,
			readOnly:  mountRequest.TypedSpec().ReadOnly,
			unmounter: unmounter,
		}
	} else if mountCtx.readOnly != mountRequest.TypedSpec().ReadOnly { // remount if needed
		var err error

		switch mountRequest.TypedSpec().ReadOnly {
		case true:
			err = mountCtx.point.RemountReadOnly()
		case false:
			err = mountCtx.point.RemountReadWrite()
		}

		if err != nil {
			return fmt.Errorf("failed to remount %q: %w", mountRequest.Metadata().ID(), err)
		}

		logger.Info("volume remounted",
			zap.String("volume", volumeStatus.Metadata().ID()),
			zap.String("read_only", fmt.Sprintf("%v -> %v", mountCtx.readOnly, mountRequest.TypedSpec().ReadOnly)),
		)

		mountCtx.readOnly = mountRequest.TypedSpec().ReadOnly
	}

	return nil
}

func (ctrl *MountController) handleOverlayMountOperation(
	logger *zap.Logger,
	mountTarget string,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	if _, ok := ctrl.activeMounts[mountRequest.Metadata().ID()]; ok {
		return nil
	}

	if volumeStatus.TypedSpec().ParentID != constants.EphemeralPartitionLabel {
		return fmt.Errorf("overlay mount is not supported for %q", volumeStatus.TypedSpec().ParentID)
	}

	mountpoint := mount.NewVarOverlay(
		[]string{mountTarget},
		mountTarget,
		mount.WithFlags(unix.MS_I_VERSION),
		mount.WithSelinuxLabel(volumeStatus.TypedSpec().MountSpec.SelinuxLabel),
	)

	unmounter, err := mountpoint.Mount(mount.WithMountPrinter(logger.Sugar().Infof))
	if err != nil {
		return fmt.Errorf("failed to mount %q: %w", mountRequest.Metadata().ID(), err)
	}

	if err = ctrl.updateTargetSettings(mountTarget, volumeStatus.TypedSpec().MountSpec); err != nil {
		unmounter() //nolint:errcheck

		return fmt.Errorf("failed to update target settings %q: %w", mountRequest.Metadata().ID(), err)
	}

	logger.Info("overlay mount",
		zap.String("volume", volumeStatus.Metadata().ID()),
		zap.String("target", mountTarget),
		zap.String("parent", volumeStatus.TypedSpec().ParentID),
	)

	ctrl.activeMounts[mountRequest.Metadata().ID()] = &mountContext{
		point:     mountpoint,
		unmounter: unmounter,
	}

	return nil
}

func (ctrl *MountController) handleSwapMountOperation(
	logger *zap.Logger,
	mountSource string,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	_, ok := ctrl.activeMounts[mountRequest.Metadata().ID()]
	if ok {
		return nil
	}

	if err := swap.On(mountSource, swap.FLAG_DISCARD_ONCE); err != nil {
		return fmt.Errorf("failed to enable swap on %q: %w", mountSource, err)
	}

	ctrl.activeMounts[mountRequest.Metadata().ID()] = &mountContext{
		point: mount.NewPoint(mountSource, "", "swap"),
	}

	logger.Info("swap enabled",
		zap.String("volume", volumeStatus.Metadata().ID()),
		zap.String("source", mountSource),
	)

	return nil
}

func (ctrl *MountController) handleUnmountOperation(
	logger *zap.Logger,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	switch volumeStatus.TypedSpec().Type {
	case block.VolumeTypeDirectory:
		return nil
	case block.VolumeTypeTmpfs:
		return fmt.Errorf("not implemented yet")
	case block.VolumeTypeDisk, block.VolumeTypePartition, block.VolumeTypeOverlay:
		if volumeStatus.TypedSpec().Filesystem == block.FilesystemTypeSwap {
			return ctrl.handleSwapUmountOperation(logger, mountRequest, volumeStatus)
		}

		return ctrl.handleDiskUnmountOperation(logger, mountRequest, volumeStatus)
	case block.VolumeTypeSymlink:
		return ctrl.handleSymlinkUmountOperation(mountRequest)
	default:
		return fmt.Errorf("unsupported volume type %q", volumeStatus.TypedSpec().Type)
	}
}

func (ctrl *MountController) handleDiskUnmountOperation(
	logger *zap.Logger,
	mountRequest *block.MountRequest,
	_ *block.VolumeStatus,
) error {
	mountCtx, ok := ctrl.activeMounts[mountRequest.Metadata().ID()]
	if !ok {
		return nil
	}

	if err := mountCtx.unmounter(); err != nil {
		return fmt.Errorf("failed to unmount %q: %w", mountRequest.Metadata().ID(), err)
	}

	delete(ctrl.activeMounts, mountRequest.Metadata().ID())

	logger.Info("volume unmount",
		zap.String("volume", mountRequest.Metadata().ID()),
		zap.String("source", mountCtx.point.Source()),
		zap.String("target", mountCtx.point.Target()),
		zap.String("filesystem", mountCtx.point.FSType()),
	)

	return nil
}

func (ctrl *MountController) handleSymlinkUmountOperation(
	mountRequest *block.MountRequest,
) error {
	delete(ctrl.activeMounts, mountRequest.Metadata().ID())

	return nil
}

func (ctrl *MountController) handleSwapUmountOperation(
	logger *zap.Logger,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	mountCtx, ok := ctrl.activeMounts[mountRequest.Metadata().ID()]
	if !ok {
		return nil
	}

	if err := swap.Off(mountCtx.point.Source()); err != nil {
		return fmt.Errorf("failed to disable swap on %q: %w", mountCtx.point.Source(), err)
	}

	delete(ctrl.activeMounts, mountRequest.Metadata().ID())

	logger.Info("swap disabled",
		zap.String("volume", volumeStatus.Metadata().ID()),
		zap.String("source", mountCtx.point.Source()),
	)

	return nil
}
