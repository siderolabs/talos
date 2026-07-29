// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// volumeMountFinalizer is the finalizer a service puts on the `VolumeMountStatus` it uses.
const volumeMountFinalizer = "service"

func (svcrunner *ServiceRunner) deleteVolumeMountRequest(ctx context.Context, requests []volumeRequest) error {
	st := svcrunner.runtime.State().V1Alpha2().Resources()

	requests = slices.Clone(requests)
	slices.Reverse(requests)

	for _, request := range requests {
		if err := st.RemoveFinalizer(ctx, block.NewVolumeMountStatus(block.NamespaceName, request.requestID).Metadata(), volumeMountFinalizer); err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to remove finalizer from mount status %q: %w", request.requestID, err)
			}
		}
	}

	activeRequests := make([]volumeRequest, 0, len(requests))

	for _, request := range requests {
		err := st.Destroy(ctx, block.NewVolumeMountRequest(block.NamespaceName, request.requestID).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to destroy volume mount request %q: %w", request.requestID, err)
		}

		if err == nil {
			activeRequests = append(activeRequests, request)
		}
	}

	for _, request := range activeRequests {
		if _, err := st.WatchFor(ctx, block.NewVolumeMountStatus(block.NamespaceName, request.requestID).Metadata(), state.WithEventTypes(state.Destroyed)); err != nil {
			return fmt.Errorf("failed to watch for volume mount status to be destroyed %q: %w", request.requestID, err)
		}
	}

	return nil
}

type volumesMountedCondition struct {
	st       state.State
	requests []volumeRequest

	mu              sync.Mutex
	pendingRequests []volumeRequest
}

func checkVolumeLifecycleEvent(event state.Event) error {
	switch event.Type {
	case state.Created, state.Updated:
		if event.Resource.Metadata().Phase() != resource.PhaseRunning {
			return fmt.Errorf("volume lifecycle is not running, cannot mount volumes")
		}
	case state.Destroyed:
		return fmt.Errorf("volume lifecycle is destroyed, cannot mount volumes")
	case state.Bootstrapped, state.Noop:
		return fmt.Errorf("unexpected event type %q for volume lifecycle", event.Type)
	case state.Errored:
		return fmt.Errorf("watch error: %w", event.Error)
	}

	return nil
}

//nolint:gocyclo
func waitForMountStatusReadyWithLifecycle(ctx context.Context, st state.State, lifecycleWatchCh <-chan state.Event, requestID string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mountStatusWatchCh := make(chan state.Event)

	if err := st.Watch(ctx, block.NewVolumeMountStatus(block.NamespaceName, requestID).Metadata(), mountStatusWatchCh); err != nil {
		return fmt.Errorf("failed to watch volume mount status %q: %w", requestID, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-lifecycleWatchCh:
			if err := checkVolumeLifecycleEvent(event); err != nil {
				return err
			}
		case event := <-mountStatusWatchCh:
			switch event.Type {
			case state.Created, state.Updated:
				if event.Resource.Metadata().Phase() == resource.PhaseRunning {
					return nil
				}
			case state.Destroyed:
				// ignore
			case state.Errored:
				return fmt.Errorf("watch error: %w", event.Error)
			case state.Bootstrapped, state.Noop:
				return fmt.Errorf("unexpected event type %q for mount status", event.Type)
			}
		}
	}
}

//nolint:gocyclo
func (cond *volumesMountedCondition) Wait(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lifecycleWatchCh := make(chan state.Event)

	if err := cond.st.Watch(ctx, block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID).Metadata(), lifecycleWatchCh); err != nil {
		return fmt.Errorf("failed to watch volume lifecycle: %w", err)
	}

	// wait for the initial watch event
	select {
	case <-ctx.Done():
		return ctx.Err()
	case event := <-lifecycleWatchCh:
		if err := checkVolumeLifecycleEvent(event); err != nil {
			return err
		}
	}

	// we mount all requests sequentially one by one
	for idx := range cond.requests {
		req := cond.requests[idx]

		// mount request IDs are stable across service restarts, so a `VolumeMountStatus` observed here
		// might still belong to the previous generation of the service, and go into tearing down phase
		// right after we observe it as running; retry until we manage to put a finalizer on a
		// `VolumeMountStatus` which is still running
		for {
			// create volume mount request
			mountRequest := block.NewVolumeMountRequest(block.NamespaceName, req.requestID)
			mountRequest.TypedSpec().Requester = req.requester
			mountRequest.TypedSpec().VolumeID = req.volumeID

			if err := cond.st.Create(ctx, mountRequest); err != nil && !state.IsConflictError(err) {
				return fmt.Errorf("failed to create mount request %q: %w", req.requestID, err)
			}

			// wait for the mount status
			if err := waitForMountStatusReadyWithLifecycle(ctx, cond.st, lifecycleWatchCh, req.requestID); err != nil {
				return err
			}

			err := cond.lockVolumeMountStatus(ctx, req.requestID)
			if err == nil {
				break
			}

			// the volume mount status went away or started tearing down after we have observed it as running,
			// so wait for the new one to be established
			if !state.IsPhaseConflictError(err) && !state.IsNotFoundError(err) {
				return err
			}
		}

		cond.mu.Lock()
		cond.pendingRequests = slices.Clone(cond.requests[idx+1:])
		cond.mu.Unlock()
	}

	return nil
}

// lockVolumeMountStatus puts the service finalizer on the volume mount status, but only if it is still running.
//
// Unlike `state.AddFinalizer`, which updates the resource in any phase, this fails with a phase conflict error
// if the volume mount status is already tearing down: putting a finalizer on it would block its teardown forever,
// as the finalizer is only removed when the service stops.
func (cond *volumesMountedCondition) lockVolumeMountStatus(ctx context.Context, requestID string) error {
	ptr := block.NewVolumeMountStatus(block.NamespaceName, requestID).Metadata()

	current, err := cond.st.Get(ctx, ptr)
	if err != nil {
		return err
	}

	_, err = cond.st.UpdateWithConflicts(
		ctx, ptr,
		func(r resource.Resource) error {
			r.Metadata().Finalizers().Add(volumeMountFinalizer)

			return nil
		},
		state.WithUpdateOwner(current.Metadata().Owner()),
		state.WithExpectedPhase(resource.PhaseRunning),
	)

	return err
}

func (cond *volumesMountedCondition) String() string {
	cond.mu.Lock()
	pendingVolumeIDs := xslices.Map(cond.pendingRequests, func(r volumeRequest) string { return r.volumeID })
	cond.mu.Unlock()

	return fmt.Sprintf("volumes %s to be mounted", strings.Join(pendingVolumeIDs, ", "))
}

// WaitForVolumesToBeMounted is a service condition that will wait for the volumes to be mounted.
func WaitForVolumesToBeMounted(st state.State, requests []volumeRequest) conditions.Condition {
	return &volumesMountedCondition{
		st:              st,
		requests:        requests,
		pendingRequests: slices.Clone(requests),
	}
}

// FindBackingVolume walks up the parent chain of a volume which is not backed by a block device of
// its own (e.g. a directory) and returns the ID of the closest ancestor volume which is, i.e. the
// volume the given one resides on.
func FindBackingVolume(ctx context.Context, st state.State, volumeID string) (string, error) {
	seen := map[string]struct{}{volumeID: {}}

	volumeStatus, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, st, volumeID)
	if err != nil {
		return "", fmt.Errorf("failed to get volume status %q: %w", volumeID, err)
	}

	for {
		// overlay volumes declare their parent directly, mounted volumes (including directories) via
		// the mount spec
		parentID := cmp.Or(volumeStatus.TypedSpec().ParentID, volumeStatus.TypedSpec().MountSpec.ParentID)
		if parentID == "" {
			return "", fmt.Errorf("volume %q is not located and doesn't reside on another volume", volumeID)
		}

		if _, ok := seen[parentID]; ok {
			return "", fmt.Errorf("cycle detected in the parent chain of volume %q at %q", volumeID, parentID)
		}

		seen[parentID] = struct{}{}

		parentStatus, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, st, parentID)
		if err != nil {
			return "", fmt.Errorf("failed to get parent volume status %q of volume %q: %w", parentID, volumeID, err)
		}

		if parentStatus.TypedSpec().Location != "" {
			return parentID, nil
		}

		volumeStatus = parentStatus
	}
}

// ResolveSystemVolumeStatuses resolves the given volume IDs to their VolumeStatus, validating that each one
// exists and is a system volume. It returns a gRPC status error suitable for returning from the API.
func ResolveSystemVolumeStatuses(ctx context.Context, coreState state.CoreState, volumeIDs []string) ([]*block.VolumeStatus, error) {
	result := make([]*block.VolumeStatus, 0, len(volumeIDs))

	for _, id := range volumeIDs {
		volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, coreState, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, status.Errorf(codes.NotFound, "volume %q not found", id)
			}

			return nil, status.Errorf(codes.Internal, "failed to get volume status with ID %q: %s", id, err)
		}

		if _, ok := volumeStatus.Metadata().Labels().Get(block.SystemVolumeLabel); !ok {
			return nil, status.Errorf(codes.InvalidArgument, "volume %q is not a system volume", id)
		}

		if volumeStatus.TypedSpec().Type != block.VolumeTypePartition {
			return nil, status.Errorf(codes.InvalidArgument, "volume %q is not a partition-backed volume (type: %v)", id, volumeStatus.TypedSpec().Type)
		}

		result = append(result, volumeStatus)
	}

	return result, nil
}

// WipeVolumesOnReboot stages CEL selectors for the given volumes to be wiped on the next boot.
func WipeVolumesOnReboot(
	ctx context.Context,
	ctrl runtime.Controller,
	logger *zap.Logger,
	volumeStatuses []*block.VolumeStatus,
) error {
	selectors, err := VolumeStatusesToSelectors(volumeStatuses)
	if err != nil {
		return fmt.Errorf("error converting volumes to selectors: %w", err)
	}

	selectorsStr, err := json.Marshal(selectors)
	if err != nil {
		return fmt.Errorf("error serializing staged volume wipe selectors: %w", err)
	}

	if ok, err := ctrl.Runtime().State().Machine().Meta().SetTag(ctx, meta.StagedWipeSelectors, string(selectorsStr)); !ok || err != nil {
		return fmt.Errorf("error adding staged partition wipe tag: %w", err)
	}

	if err := ctrl.Runtime().State().Machine().Meta().Flush(); err != nil {
		return fmt.Errorf("error writing meta: %w", err)
	}

	logger.Sugar().Infof("staged %d volume(s) for wipe on next boot; CEL selectors: %q", len(volumeStatuses), selectorsStr)

	return nil
}

// WipeVolumesNow immediately wipes each of the given volumes, failing fast on the first error.
func WipeVolumesNow(
	ctx context.Context,
	ctrl runtime.Controller,
	logger *zap.Logger,
	volumeStatuses []*block.VolumeStatus,
) error {
	for _, volumeStatus := range volumeStatuses {
		if volumeStatus.TypedSpec().Location == "" {
			return fmt.Errorf(
				"volume %q is not located",
				volumeStatus.Metadata().ID(),
			)
		}

		target := partition.VolumeWipeTargetFromVolumeStatus(volumeStatus)

		if err := target.Wipe(ctx, log.Printf); err != nil {
			return fmt.Errorf(
				"failed to wipe volume %q: %s; if the volume is in use, retry with --on-reboot",
				volumeStatus.Metadata().ID(),
				err,
			)
		}
	}

	return nil
}

// AssertVolumesNotMounted rejects an immediate wipe of any volume that is currently mounted (in use).
//
// A mounted volume can't be wiped safely while the node is running; that's what --on-reboot is for.
// Mount state is tracked by block.VolumeMountStatus resources, keyed to a volume via VolumeID.
func AssertVolumesNotMounted(ctx context.Context, ctrl runtime.Controller, ids []string) error {
	mountStatuses, err := safe.StateListAll[*block.VolumeMountStatus](ctx, ctrl.Runtime().State().V1Alpha2().Resources())
	if err != nil {
		return status.Errorf(codes.Internal, "failed to list volume mount statuses: %s", err)
	}

	wanted := xslices.ToSet(ids)

	for mountStatus := range mountStatuses.All() {
		if _, ok := wanted[mountStatus.TypedSpec().VolumeID]; ok {
			return status.Errorf(codes.FailedPrecondition,
				"volume %q is in use (mounted); retry with --on-reboot", mountStatus.TypedSpec().VolumeID)
		}
	}

	return nil
}

// VolumeStatusesToSelectors converts each volume status to a CEL selector matching that volume.
func VolumeStatusesToSelectors(volumeStatuses []*block.VolumeStatus) ([]cel.Expression, error) {
	selectors := make([]cel.Expression, 0, len(volumeStatuses))

	for _, volumeStatus := range volumeStatuses {
		selector, err := VolumeStatusToSelector(volumeStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to convert volume %q to selector: %w", volumeStatus.Metadata().ID(), err)
		}

		selectors = append(selectors, selector)
	}

	return selectors, nil
}

// VolumeStatusToSelector builds a CEL selector that uniquely matches the given volume.
//
// Only partition-backed volumes are supported for now; the partition UUID is a stable
// identifier that survives a reboot, so it's used to locate the volume again on next boot.
func VolumeStatusToSelector(volumeStatus *block.VolumeStatus) (cel.Expression, error) {
	spec := volumeStatus.TypedSpec()

	switch spec.Type { //nolint:exhaustive
	case block.VolumeTypePartition:
		if spec.PartitionUUID == "" {
			return cel.Expression{}, fmt.Errorf("volume %q has no partition UUID", volumeStatus.Metadata().ID())
		}

		// build `volume.partition_uuid == "<uuid>"` programmatically so the UUID is a
		// typed string literal rather than interpolated into the expression text
		builder := cel.NewBuilder(celenv.VolumeLocator())

		expr := builder.NewCall(
			builder.NextID(),
			operators.Equals,
			builder.NewSelect(
				builder.NextID(),
				builder.NewIdent(builder.NextID(), "volume"),
				"partition_uuid",
			),
			builder.NewLiteral(builder.NextID(), types.String(spec.PartitionUUID)),
		)

		selector, err := builder.ToBooleanExpression(expr)
		if err != nil {
			return cel.Expression{}, fmt.Errorf("failed to build wipe selector for volume %q: %w", volumeStatus.Metadata().ID(), err)
		}

		return *selector, nil
	default:
		return cel.Expression{}, fmt.Errorf("volume %q: wipe selector unsupported for volume type %s", volumeStatus.Metadata().ID(), spec.Type)
	}
}
