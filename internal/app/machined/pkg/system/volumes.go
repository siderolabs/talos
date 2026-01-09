// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func (svcrunner *ServiceRunner) deleteVolumeMountRequest(ctx context.Context, requests []volumeRequest) error {
	st := svcrunner.runtime.State().V1Alpha2().Resources()

	requests = slices.Clone(requests)
	slices.Reverse(requests)

	for _, request := range requests {
		if err := st.RemoveFinalizer(ctx, block.NewVolumeMountStatus(request.requestID).Metadata(), "service"); err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to remove finalizer from mount status %q: %w", request.requestID, err)
			}
		}
	}

	for _, request := range requests {
		err := st.Destroy(ctx, block.NewVolumeMountRequest(block.NamespaceName, request.requestID).Metadata())
		if err != nil {
			return fmt.Errorf("failed to destroy volume mount request %q: %w", request.requestID, err)
		}
	}

	for _, request := range requests {
		if _, err := st.WatchFor(ctx, block.NewVolumeMountStatus(request.requestID).Metadata(), state.WithEventTypes(state.Destroyed)); err != nil {
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

		// create volume mount request
		mountRequest := block.NewVolumeMountRequest(block.NamespaceName, req.requestID)
		mountRequest.TypedSpec().Requester = req.requester
		mountRequest.TypedSpec().VolumeID = req.volumeID

		if err := cond.st.Create(ctx, mountRequest); err != nil {
			if !state.IsConflictError(err) {
				return fmt.Errorf("failed to create mount request: %w", err)
			}
		}

		// wait for the mount status
		_, err := cond.st.WatchFor(ctx,
			block.NewVolumeMountStatus(req.requestID).Metadata(),
			state.WithEventTypes(state.Created, state.Updated),
			state.WithPhases(resource.PhaseRunning),
		)
		if err != nil {
			return err
		}

		if err = cond.st.AddFinalizer(ctx, block.NewVolumeMountStatus(req.requestID).Metadata(), "service"); err != nil {
			return err
		}

		cond.mu.Lock()
		cond.pendingRequests = slices.Clone(cond.requests[idx+1:])
		cond.mu.Unlock()
	}

	return nil
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
