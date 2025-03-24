// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/mount/v2"
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

				if mountHasParent {
					// mount target is a path within the parent mount
					mountTarget = filepath.Join(mountParentStatus.TypedSpec().Target, mountTarget)
				}

				if err = ctrl.handleMountOperation(logger, mountSource, mountTarget, mountFilesystem, mountRequest, volumeStatus); err != nil {
					return err
				}

				if err = safe.WriterModify(
					ctx, r, block.NewMountStatus(block.NamespaceName, mountRequest.Metadata().ID()),
					func(mountStatus *block.MountStatus) error {
						mountStatus.TypedSpec().Spec = *mountRequest.TypedSpec()
						mountStatus.TypedSpec().Source = mountSource
						mountStatus.TypedSpec().Target = mountTarget
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
	mountSource, mountTarget string,
	mountFilesystem block.FilesystemType,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	switch volumeStatus.TypedSpec().Type {
	case block.VolumeTypeDirectory:
		return ctrl.handleDirectoryMountOperation(mountTarget, volumeStatus)
	case block.VolumeTypeTmpfs:
		return fmt.Errorf("not implemented yet")
	case block.VolumeTypeDisk, block.VolumeTypePartition:
		return ctrl.handleDiskMountOperation(logger, mountSource, mountTarget, mountFilesystem, mountRequest, volumeStatus)
	default:
		return fmt.Errorf("unsupported volume type %q", volumeStatus.TypedSpec().Type)
	}
}

func (ctrl *MountController) handleDirectoryMountOperation(
	_ string,
	_ *block.VolumeStatus,
) error {
	// [TODO]: implement me
	//  - create directory if missing
	//  - set SELinux label if needed
	//  - set uid:gid if needed
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

func (ctrl *MountController) handleUnmountOperation(
	logger *zap.Logger,
	mountRequest *block.MountRequest,
	volumeStatus *block.VolumeStatus,
) error {
	switch volumeStatus.TypedSpec().Type {
	case block.VolumeTypeDirectory:
		return ctrl.handleDirectoryUnmountOperation(mountRequest, volumeStatus)
	case block.VolumeTypeTmpfs:
		return fmt.Errorf("not implemented yet")
	case block.VolumeTypeDisk, block.VolumeTypePartition:
		return ctrl.handleDiskUnmountOperation(logger, mountRequest, volumeStatus)
	default:
		return fmt.Errorf("unsupported volume type %q", volumeStatus.TypedSpec().Type)
	}
}

func (ctrl *MountController) handleDirectoryUnmountOperation(
	_ *block.MountRequest,
	_ *block.VolumeStatus,
) error {
	return nil
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
