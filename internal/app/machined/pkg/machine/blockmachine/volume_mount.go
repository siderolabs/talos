// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blockmachine

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeMountCallbackFunc is a callback function that is called when a volume is mounted.
type VolumeMountCallbackFunc func(context.Context, controller.ReaderWriter, *zap.Logger, *block.VolumeMountStatus) error

// volumeMountContext is the internal context for the volume mounter controller state machine.
type volumeMountContext struct {
	mountID   string
	volumeID  string
	requester string
	callback  VolumeMountCallbackFunc
}

// VolumeMounterMachine is the type of the volume mounter controller state machine.
type VolumeMounterMachine = *machine.ControllerMachine[volumeMountContext]

// NewVolumeMounter creates a new volume mounter controller state machine.
//
// It ensures that the volume is mounted, and calls the callback function when the volume is mounted,
// unmounting the volume before terminating the state machine.
func NewVolumeMounter(requester, volumeID string, callback VolumeMountCallbackFunc) VolumeMounterMachine {
	return machine.NewControllerMachine(createVolumeMountRequest,
		volumeMountContext{
			mountID:   requester + "-" + volumeID,
			volumeID:  volumeID,
			requester: requester,
			callback:  callback,
		},
	)
}

// createVolumeMountRequest is the initial state of the volume mounter controller state machine.
//
// Transitions to: waitForMountStatus.
func createVolumeMountRequest(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountContext volumeMountContext) (machine.ControllerStateFunc[volumeMountContext], error) {
	if err := safe.WriterModify(ctx, r, block.NewVolumeMountRequest(block.NamespaceName, mountContext.mountID), func(req *block.VolumeMountRequest) error {
		req.TypedSpec().VolumeID = mountContext.volumeID
		req.TypedSpec().Requester = mountContext.requester

		return nil
	}); err != nil {
		return nil, fmt.Errorf("error creating volume mount request: %w", err)
	}

	return waitForMountStatus, nil
}

// waitForMountStatus is the state of the volume mounter controller state machine that waits for the mount status to be established.
//
// Transitions to: callbackWithMountStatus.
func waitForMountStatus(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountContext volumeMountContext) (machine.ControllerStateFunc[volumeMountContext], error) {
	mountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, mountContext.mountID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error reading volume mount status: %w", err)
	}

	if mountStatus == nil {
		// wait for the mount status to be established
		return nil, xerrors.NewTaggedf[machine.Continue]("waiting for mount status to be established")
	}

	if !mountStatus.Metadata().Finalizers().Has(mountContext.requester) {
		if err = r.AddFinalizer(ctx, mountStatus.Metadata(), mountContext.requester); err != nil {
			return nil, fmt.Errorf("error adding finalizer: %w", err)
		}
	}

	return callbackWithMountStatus(mountStatus), nil
}

// callbackWithMountStatus is the state of the volume mounter controller state machine that calls the callback with the mount status.
//
// Transitions to: removeMountStatusFinalizer.
func callbackWithMountStatus(mountStatus *block.VolumeMountStatus) func(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountContext volumeMountContext,
) (machine.ControllerStateFunc[volumeMountContext], error) {
	return func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountContext volumeMountContext) (machine.ControllerStateFunc[volumeMountContext], error) {
		if err := mountContext.callback(ctx, r, logger, mountStatus); err != nil {
			return nil, fmt.Errorf("error calling callback: %w", err)
		}

		return removeMountStatusFinalizer, nil
	}
}

// removeMountStatusFinalizer is the state of the volume mounter controller state machine that removes the mount status finalizer.
//
// Transitions to: removeMountRequest.
func removeMountStatusFinalizer(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountContext volumeMountContext) (machine.ControllerStateFunc[volumeMountContext], error) {
	if err := r.RemoveFinalizer(ctx, block.NewVolumeMountStatus(block.NamespaceName, mountContext.mountID).Metadata(), mountContext.requester); err != nil {
		return nil, fmt.Errorf("error removing finalizer: %w", err)
	}

	return removeMountRequest, nil
}

// removeMountRequest is the state of the volume mounter controller state machine that removes the mount request.
//
// Transitions to: nil.
func removeMountRequest(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountContext volumeMountContext) (machine.ControllerStateFunc[volumeMountContext], error) {
	mountRequest := block.NewVolumeMountRequest(block.NamespaceName, mountContext.mountID)

	okToDestroy, err := r.Teardown(ctx, mountRequest.Metadata())
	if err != nil {
		return nil, fmt.Errorf("error tearing down mount request: %w", err)
	}

	if !okToDestroy {
		return nil, xerrors.NewTaggedf[machine.Continue]("mount request is not ready to be destroyed")
	}

	if err = r.Destroy(ctx, mountRequest.Metadata()); err != nil {
		return nil, fmt.Errorf("error destroying mount request: %w", err)
	}

	return nil, nil
}
