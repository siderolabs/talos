// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/conditions"
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
