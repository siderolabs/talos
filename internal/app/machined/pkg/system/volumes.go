// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
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
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/conditions"
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

func (cond *volumesMountedCondition) Wait(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
			_, err := cond.st.WatchFor(
				ctx,
				block.NewVolumeMountStatus(block.NamespaceName, req.requestID).Metadata(),
				state.WithEventTypes(state.Created, state.Updated),
				state.WithPhases(resource.PhaseRunning),
			)
			if err != nil {
				return err
			}

			err = cond.lockVolumeMountStatus(ctx, req.requestID)
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

// WipeVolumesOnReboot marks the partition UUIDs to be wiped on the next boot.
func WipeVolumesOnReboot(ctx context.Context, ctrl runtime.Controller, volumeStatuses []*block.VolumeStatus) error {
	partitionUUIDs := make([]string, 0, len(volumeStatuses))
	for _, volumeStatus := range volumeStatuses {
		partitionUUIDs = append(partitionUUIDs, volumeStatus.TypedSpec().PartitionUUID)
	}

	serializedPartitionUUIDs, err := json.Marshal(partitionUUIDs)
	if err != nil {
		return fmt.Errorf("error serializing staged partition UUIDs: %w", err)
	}

	if ok, err := ctrl.Runtime().State().Machine().Meta().SetTag(ctx, meta.StagedPartitionsToWipe, string(serializedPartitionUUIDs)); !ok || err != nil {
		return fmt.Errorf("error adding staged partition wipe tag: %w", err)
	}

	if err := ctrl.Runtime().State().Machine().Meta().Flush(); err != nil {
		return fmt.Errorf("error writing meta: %w", err)
	}

	return nil
}

// WipeVolumesNow immediately wipes each of the given volumes, failing fast on the first error.
func WipeVolumesNow(ctx context.Context, ctrl runtime.Controller, volumeStatuses []*block.VolumeStatus) error {
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
