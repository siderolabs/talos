// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// containerLabel carries the owning container on the mount requests this controller creates.
//
// The request ID is not parsed for it: a request that is no longer wanted still has to be attributed
// to a container to decide whether releasing it is safe.
const containerLabel = "container"

// MountController resolves a container's mounts to host paths, and holds the volumes they need.
//
// Its one side effect is the block.VolumeMountRequest resources it creates and the finalizers it
// holds on the resulting block.VolumeMountStatus. That finalizer is what stops a volume being
// unmounted from under a running container, so it is released only once nothing is running.
type MountController struct{}

// Name implements controller.Controller interface.
func (ctrl *MountController) Name() string {
	return "containers.MountController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MountController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerSpecType,
			Kind:      controller.InputWeak,
		},
		// Both instance types are read to decide whether a container is still using its mounts: a
		// status alone leaves a hole between an instance being created and its first status being
		// written, during which a starting container looks idle.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerLifecycleType,
			ID:        optional.Some(containers.ContainerLifecycleID),
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			// InputDestroyReady is what lets this controller tear down its own mount requests: it
			// wakes us when one is tearing down with no finalizers left.
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MountController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerMountStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			// Shared: many controllers write mount requests.
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MountController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			logger.Error("failed to reconcile container mounts", zap.Error(err))

			return err
		}

		r.ResetRestartBackoff()
	}
}

// mountRequestID builds the ID of the mount request for one container and volume.
//
// Per container rather than per volume, so two containers sharing a volume hold it independently and
// one stopping does not release the other's mount. Slash-separated because container and volume names
// both contain hyphens, which would make a hyphen-joined ID ambiguous and let two containers collide
// on one request.
func (ctrl *MountController) mountRequestID(containerID, volumeID string) string {
	return ctrl.Name() + "/" + containerID + "/" + volumeID
}

//nolint:gocyclo
func (ctrl *MountController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	lifecycle, err := readContainerLifecycle(ctx, r)
	if err != nil {
		return err
	}

	// The barrier tearing down, or being gone, is the node on its way down. Everything is released
	// here and nothing is requested again. Releasing without waiting for containers to stop is safe
	// because it does not unmount anything by itself: the stopContainers phase runs before the
	// unmount phases, so the volumes are still mounted for as long as the containers need them.
	if lifecycle == nil || lifecycle.Metadata().Phase() == resource.PhaseTearingDown {
		held, err := ctrl.releaseAll(ctx, r, logger)
		if err != nil {
			return err
		}

		return reconcileLifecycle(ctx, r, logger, lifecycle, ctrl.Name(), held == 0)
	}

	specs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list container specs: %w", err)
	}

	live, err := ctrl.liveContainers(ctx, r)
	if err != nil {
		return err
	}

	r.StartTrackingOutputs()

	wanted := map[string]struct{}{}

	for spec := range specs.All() {
		containerID := spec.Metadata().ID()

		resolved, ready, reason, err := ctrl.reconcileContainer(ctx, r, logger, spec, wanted)
		if err != nil {
			return err
		}

		if err := safe.WriterModify(ctx, r,
			containers.NewContainerMountStatus(containers.NamespaceName, containerID),
			func(res *containers.ContainerMountStatus) error {
				res.TypedSpec().Ready = ready
				res.TypedSpec().Mounts = resolved
				res.TypedSpec().Error = reason

				return nil
			},
		); err != nil {
			return fmt.Errorf("failed to write mount status %q: %w", containerID, err)
		}
	}

	if err := ctrl.releaseUnwanted(ctx, r, logger, wanted, live); err != nil {
		return err
	}

	if err := safe.CleanupOutputs[*containers.ContainerMountStatus](ctx, r); err != nil {
		return fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return reconcileLifecycle(ctx, r, logger, lifecycle, ctrl.Name(), len(wanted) == 0)
}

// liveContainers returns the containers which may still be using their mounts.
func (ctrl *MountController) liveContainers(ctx context.Context, r controller.Runtime) (map[string]struct{}, error) {
	live := map[string]struct{}{}

	instances, err := safe.ReaderListAll[*containers.ContainerInstanceSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance specs: %w", err)
	}

	// An instance that exists is either running or on its way to it, so its mounts are in use even
	// before any status has been written.
	for instance := range instances.All() {
		live[instance.TypedSpec().ContainerID] = struct{}{}
	}

	statuses, err := safe.ReaderListAll[*containers.ContainerInstanceStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance statuses: %w", err)
	}

	for status := range statuses.All() {
		if !status.TypedSpec().Phase.Done() {
			live[status.TypedSpec().ContainerID] = struct{}{}
		}
	}

	return live, nil
}

// reconcileContainer requests the mounts one container needs and resolves them to host paths.
//
//nolint:gocyclo,cyclop
func (ctrl *MountController) reconcileContainer(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	spec *containers.ContainerSpec,
	wanted map[string]struct{},
) (resolved []containers.ResolvedMountSpec, ready bool, reason string, err error) {
	containerID := spec.Metadata().ID()
	ready = true

	var reasons []string

	for _, mount := range spec.TypedSpec().Mounts {
		if mount.Kind != containers.MountKindUserVolume {
			// tmpfs and hostPath need nothing from the block subsystem: the source is either nothing
			// at all or a path which must already exist.
			resolved = append(resolved, containers.ResolvedMountSpec{
				Kind:        mount.Kind,
				Source:      mount.Source,
				Destination: mount.Destination,
				Size:        mount.Size,
				Options:     mount.Options,
			})

			continue
		}

		requestID := ctrl.mountRequestID(containerID, mount.VolumeID)
		wanted[requestID] = struct{}{}

		// Writable unless the options say otherwise; the writable default is already applied by
		// ConfigController.
		readOnly := slices.Contains(mount.Options, "ro")

		if err = safe.WriterModify(ctx, r,
			block.NewVolumeMountRequest(block.NamespaceName, requestID),
			func(res *block.VolumeMountRequest) error {
				res.Metadata().Labels().Set(containerLabel, containerID)

				res.TypedSpec().VolumeID = mount.VolumeID
				res.TypedSpec().Requester = ctrl.Name()
				res.TypedSpec().ReadOnly = readOnly
				// Detached must stay false: a detached mount is reachable only through a file
				// descriptor and never appears at its target, so there would be no path to bind.
				res.TypedSpec().Detached = false

				return nil
			},
		); err != nil {
			return nil, false, "", fmt.Errorf("failed to write mount request %q: %w", requestID, err)
		}

		mountStatus, getErr := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, requestID)
		if getErr != nil {
			if !state.IsNotFoundError(getErr) {
				return nil, false, "", fmt.Errorf("failed to get mount status %q: %w", requestID, getErr)
			}

			ready = false

			reasons = append(reasons, fmt.Sprintf("waiting for volume %q to be mounted", mount.VolumeID))

			continue
		}

		if mountStatus.Metadata().Phase() != resource.PhaseRunning {
			// Either the volume is going away, or this status is left over from a previous generation
			// and is still tearing down. Adding a finalizer now would block that teardown forever, so
			// report not-ready instead so the container won't start/restart while the volume is unavailable.
			ready = false

			reasons = append(reasons, fmt.Sprintf("volume %q is being unmounted", mount.VolumeID))

			continue
		}

		if mountStatus.TypedSpec().ReadOnly && !readOnly {
			// Mount requests are merged per volume and end up read-only if every requester asked for
			// read-only, so another holder can leave this one with less access than it asked for.
			ready = false

			reasons = append(reasons, fmt.Sprintf("volume %q is mounted read-only", mount.VolumeID))

			continue
		}

		if !mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			if err = r.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
				return nil, false, "", fmt.Errorf("failed to add finalizer on %q: %w", requestID, err)
			}

			logger.Info("holding volume mount for container",
				zap.String("container", containerID),
				zap.String("volume", mount.VolumeID),
				zap.String("target", mountStatus.TypedSpec().Target),
				zap.Bool("readOnly", readOnly),
			)
		}

		resolved = append(resolved, containers.ResolvedMountSpec{
			Kind:        mount.Kind,
			Source:      mountStatus.TypedSpec().Target,
			Destination: mount.Destination,
			Options:     mount.Options,
			VolumeID:    mount.VolumeID,
		})
	}

	return resolved, ready, strings.Join(reasons, "; "), nil
}

// releaseUnwanted releases the mounts no container needs any more.
func (ctrl *MountController) releaseUnwanted(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	wanted map[string]struct{},
	live map[string]struct{},
) error {
	ours, err := ctrl.ownedRequests(ctx, r)
	if err != nil {
		return err
	}

	for _, request := range ours {
		requestID := request.Metadata().ID()

		if _, stillWanted := wanted[requestID]; stillWanted {
			continue
		}

		containerID, _ := request.Metadata().Labels().Get(containerLabel)

		// The spec may no longer list the mount while the task still has the path open, so the hold
		// outlives the request for as long as anything is running.
		if _, isLive := live[containerID]; isLive {
			logger.Debug("deferring volume mount release until the container stops",
				zap.String("container", containerID),
				zap.String("request", requestID),
			)

			continue
		}

		if err := ctrl.release(ctx, r, logger, requestID); err != nil {
			return err
		}
	}

	return nil
}

// releaseAll releases every mount this controller holds, reporting how many are still held.
func (ctrl *MountController) releaseAll(ctx context.Context, r controller.Runtime, logger *zap.Logger) (int, error) {
	ours, err := ctrl.ownedRequests(ctx, r)
	if err != nil {
		return 0, err
	}

	for _, request := range ours {
		if err := ctrl.release(ctx, r, logger, request.Metadata().ID()); err != nil {
			return 0, err
		}
	}

	// Counted after the pass: a request whose teardown is still waiting on another finalizer is
	// still held, and the shutdown barrier has to keep waiting for it.
	remaining, err := ctrl.ownedRequests(ctx, r)
	if err != nil {
		return 0, err
	}

	return len(remaining), nil
}

// ownedRequests returns the mount requests created by this controller.
func (ctrl *MountController) ownedRequests(ctx context.Context, r controller.Runtime) ([]*block.VolumeMountRequest, error) {
	requests, err := safe.ReaderListAll[*block.VolumeMountRequest](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list mount requests: %w", err)
	}

	var ours []*block.VolumeMountRequest

	for request := range requests.All() {
		if request.TypedSpec().Requester == ctrl.Name() {
			ours = append(ours, request)
		}
	}

	return ours, nil
}

// release gives back one mount: the finalizer first, then the request itself.
func (ctrl *MountController) release(ctx context.Context, r controller.Runtime, logger *zap.Logger, requestID string) error {
	if err := ctrl.releaseFinalizer(ctx, r, logger, requestID); err != nil {
		return err
	}

	return ctrl.destroyRequest(ctx, r, logger, requestID)
}

// releaseFinalizer drops this controller's hold on the mount status, if it holds one.
func (ctrl *MountController) releaseFinalizer(ctx context.Context, r controller.Runtime, logger *zap.Logger, requestID string) error {
	mountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, requestID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get mount status %q: %w", requestID, err)
	}

	if !mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	if err := r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("failed to remove finalizer on %q: %w", requestID, err)
	}

	logger.Info("released volume mount", zap.String("request", requestID))

	return nil
}

// destroyRequest tears down the mount request and destroys it once nothing holds it.
func (ctrl *MountController) destroyRequest(ctx context.Context, r controller.Runtime, logger *zap.Logger, requestID string) error {
	requestMD := block.NewVolumeMountRequest(block.NamespaceName, requestID).Metadata()

	okToDestroy, err := r.Teardown(ctx, requestMD)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to tear down mount request %q: %w", requestID, err)
	}

	if !okToDestroy {
		logger.Debug("waiting for the volume mount request to be released", zap.String("request", requestID))

		return nil
	}

	if err := r.Destroy(ctx, requestMD); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to destroy mount request %q: %w", requestID, err)
	}

	return nil
}
