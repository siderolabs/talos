// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// deviceHoldFinalizer returns the finalizer holding a raw volume for a container.
//
// A raw volume is never mounted, so it is held directly on its VolumeStatus.
func (ctrl *MountController) deviceHoldFinalizer(containerSpecID string) string {
	return ctrl.deviceHoldPrefix() + containerSpecID
}

// deviceHoldPrefix is the part shared by all raw volume holds of this controller.
func (ctrl *MountController) deviceHoldPrefix() string {
	return ctrl.Name() + "/"
}

// deviceHolders returns the containers holding the raw volume.
func (ctrl *MountController) deviceHolders(volumeStatus *block.VolumeStatus) []string {
	var holders []string

	for _, finalizer := range *volumeStatus.Metadata().Finalizers() {
		if containerSpecID, ok := strings.CutPrefix(finalizer, ctrl.deviceHoldPrefix()); ok {
			holders = append(holders, containerSpecID)
		}
	}

	return holders
}

// deviceUsers returns, per raw volume ID, the containers with a live instance using it.
func (ctrl *MountController) deviceUsers(ctx context.Context, runtime controller.Runtime) (map[string]map[string]struct{}, error) {
	containerInstanceStatuses, err := safe.ReaderListAll[*containers.ContainerInstanceStatus](ctx, runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance statuses: %w", err)
	}

	done := map[string]struct{}{}

	for containerInstanceStatus := range containerInstanceStatuses.All() {
		if containerInstanceStatus.TypedSpec().Phase.Done() {
			done[containerInstanceStatus.Metadata().ID()] = struct{}{}
		}
	}

	containerInstanceSpecs, err := safe.ReaderListAll[*containers.ContainerInstanceSpec](ctx, runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance specs: %w", err)
	}

	users := map[string]map[string]struct{}{}

	for containerInstanceSpec := range containerInstanceSpecs.All() {
		if _, isDone := done[containerInstanceSpec.Metadata().ID()]; isDone {
			continue
		}

		for _, device := range containerInstanceSpec.TypedSpec().Devices {
			if users[device.VolumeID] == nil {
				users[device.VolumeID] = map[string]struct{}{}
			}

			users[device.VolumeID][containerInstanceSpec.TypedSpec().ContainerID] = struct{}{}
		}
	}

	return users, nil
}

// reconcileContainerDevices holds the raw volumes one container needs and resolves them to device nodes.
//
//nolint:gocyclo,cyclop
func (ctrl *MountController) reconcileContainerDevices(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpec *containers.ContainerSpec,
	users map[string]map[string]struct{},
	wantedHolds map[string]map[string]struct{},
) (resolvedDevices []containers.ResolvedDeviceSpec, ready bool, reasons []string, err error) {
	containerSpecID := containerSpec.Metadata().ID()
	finalizer := ctrl.deviceHoldFinalizer(containerSpecID)
	ready = true

	for _, containerDeviceSpec := range containerSpec.TypedSpec().Devices {
		if containerDeviceSpec.Kind != containers.DeviceKindRawVolume {
			return nil, false, nil, fmt.Errorf("unsupported device kind %q", containerDeviceSpec.Kind)
		}

		volumeName := containerDeviceSpec.VolumeName
		volumeID := constants.RawVolumePrefix + volumeName

		volumeStatus, getErr := safe.ReaderGetByID[*block.VolumeStatus](ctx, runtime, volumeID)
		if getErr != nil {
			if !state.IsNotFoundError(getErr) {
				return nil, false, nil, fmt.Errorf("failed to get volume status %q: %w", volumeID, getErr)
			}

			ready = false

			reasons = append(reasons, fmt.Sprintf("waiting for raw volume %q to be ready", volumeName))

			continue
		}

		held := volumeStatus.Metadata().Finalizers().Has(finalizer)

		if volumeStatus.Metadata().Phase() != resource.PhaseRunning {
			ready = false

			if _, inUse := users[volumeID][containerSpecID]; held && inUse {
				reasons = append(reasons, fmt.Sprintf("raw volume %q is being closed, waiting for the container to stop", volumeName))
			} else {
				reasons = append(reasons, fmt.Sprintf("raw volume %q is being closed", volumeName))
			}

			continue
		}

		if otherHolder := ctrl.otherDeviceHolder(volumeStatus, containerSpecID); otherHolder != "" {
			ready = false

			reasons = append(reasons, fmt.Sprintf("raw volume %q is held by container %q", volumeName, otherHolder))

			continue
		}

		if wantedHolds[volumeID] == nil {
			wantedHolds[volumeID] = map[string]struct{}{}
		}

		wantedHolds[volumeID][containerSpecID] = struct{}{}

		if !held {
			if err = runtime.AddFinalizer(ctx, volumeStatus.Metadata(), finalizer); err != nil {
				return nil, false, nil, fmt.Errorf("failed to add finalizer on %q: %w", volumeID, err)
			}

			// A finalizer can be added to a status which has just started tearing down.
			if volumeStatus, err = safe.ReaderGetByID[*block.VolumeStatus](ctx, runtime, volumeID); err != nil {
				return nil, false, nil, fmt.Errorf("failed to get volume status %q: %w", volumeID, err)
			}

			if volumeStatus.Metadata().Phase() != resource.PhaseRunning || ctrl.otherDeviceHolder(volumeStatus, containerSpecID) != "" {
				delete(wantedHolds[volumeID], containerSpecID)

				ready = false

				reasons = append(reasons, fmt.Sprintf("raw volume %q is not available", volumeName))

				continue
			}

			logger.Debug("holding raw volume for container",
				zap.String("container", containerSpecID),
				zap.String("volume", volumeName),
				zap.String("volumeID", volumeID),
			)
		}

		if volumeStatus.TypedSpec().Phase != block.VolumePhaseReady || volumeStatus.TypedSpec().MountLocation == "" {
			ready = false

			reasons = append(reasons, fmt.Sprintf("waiting for raw volume %q to be ready", volumeName))

			continue
		}

		resolvedDevices = append(resolvedDevices, containers.ResolvedDeviceSpec{
			Kind:        containerDeviceSpec.Kind,
			Source:      volumeStatus.TypedSpec().MountLocation,
			Destination: containerDeviceSpec.Destination,
			VolumeID:    volumeID,
			VolumeName:  volumeName,
		})
	}

	return resolvedDevices, ready, reasons, nil
}

// otherDeviceHolder returns a container other than containerSpecID holding the raw volume, if any.
func (ctrl *MountController) otherDeviceHolder(volumeStatus *block.VolumeStatus, containerSpecID string) string {
	for _, holder := range ctrl.deviceHolders(volumeStatus) {
		if holder != containerSpecID {
			return holder
		}
	}

	return ""
}

// releaseUnwantedDevices releases the raw volume holds which are neither wanted nor in use.
//
// It must run after the mount statuses are written: an instance created before is seen as a user, one
// created after is refused by RuntimeController.refuseStale.
func (ctrl *MountController) releaseUnwantedDevices(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	wantedHolds map[string]map[string]struct{},
) error {
	users, err := ctrl.deviceUsers(ctx, runtime)
	if err != nil {
		return err
	}

	return ctrl.releaseDevices(ctx, runtime, logger, func(volumeID, containerSpecID string) bool {
		_, wanted := wantedHolds[volumeID][containerSpecID]
		_, inUse := users[volumeID][containerSpecID]

		return !wanted && !inUse
	})
}

// releaseDevices releases the raw volume holds for which release returns true.
func (ctrl *MountController) releaseDevices(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	release func(volumeID, containerSpecID string) bool,
) error {
	volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, runtime,
		state.WithLabelQuery(resource.LabelExists(block.RawVolumeLabel)),
	)
	if err != nil {
		return fmt.Errorf("failed to list raw volume statuses: %w", err)
	}

	for volumeStatus := range volumeStatuses.All() {
		volumeID := volumeStatus.Metadata().ID()

		for _, containerSpecID := range ctrl.deviceHolders(volumeStatus) {
			if !release(volumeID, containerSpecID) {
				continue
			}

			if err = runtime.RemoveFinalizer(ctx, volumeStatus.Metadata(), ctrl.deviceHoldFinalizer(containerSpecID)); err != nil {
				return fmt.Errorf("failed to remove finalizer on %q: %w", volumeID, err)
			}

			logger.Debug("released raw volume", zap.String("container", containerSpecID), zap.String("volumeID", volumeID))
		}
	}

	return nil
}

// heldDevices counts the raw volume holds of this controller.
func (ctrl *MountController) heldDevices(ctx context.Context, runtime controller.Runtime) (int, error) {
	volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, runtime,
		state.WithLabelQuery(resource.LabelExists(block.RawVolumeLabel)),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to list raw volume statuses: %w", err)
	}

	held := 0

	for volumeStatus := range volumeStatuses.All() {
		held += len(ctrl.deviceHolders(volumeStatus))
	}

	return held, nil
}

// releaseAllDevices releases every raw volume hold on the way down, reporting how many are left.
//
// Unlike a mount, a raw volume being closed closes as soon as it is released, so the holds are kept
// until RuntimeController has stopped every container and released the shutdown barrier.
func (ctrl *MountController) releaseAllDevices(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerLifecycle *containers.ContainerLifecycle,
) (int, error) {
	if containerLifecycle != nil && containerLifecycle.Metadata().Finalizers().Has((&RuntimeController{}).Name()) {
		return ctrl.heldDevices(ctx, runtime)
	}

	if err := ctrl.releaseDevices(ctx, runtime, logger, func(string, string) bool { return true }); err != nil {
		return 0, err
	}

	return ctrl.heldDevices(ctx, runtime)
}
