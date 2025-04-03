// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func (svcrunner *ServiceRunner) createVolumeMountRequest(ctx context.Context, volumeID string) (string, error) {
	st := svcrunner.runtime.State().V1Alpha2().Resources()
	requester := "service/" + svcrunner.id
	requestID := requester + "-" + volumeID

	mountRequest := block.NewVolumeMountRequest(block.NamespaceName, requestID)
	mountRequest.TypedSpec().Requester = requester
	mountRequest.TypedSpec().VolumeID = volumeID

	if err := st.Create(ctx, mountRequest); err != nil {
		if !state.IsConflictError(err) {
			return "", fmt.Errorf("failed to create mount request: %w", err)
		}
	}

	return requestID, nil
}

func (svcrunner *ServiceRunner) deleteVolumeMountRequest(ctx context.Context, requests []volumeRequest) error {
	st := svcrunner.runtime.State().V1Alpha2().Resources()

	for _, request := range requests {
		if err := st.RemoveFinalizer(ctx, block.NewVolumeMountStatus(block.NamespaceName, request.requestID).Metadata(), "service"); err != nil {
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
		if _, err := st.WatchFor(ctx, block.NewVolumeMountStatus(block.NamespaceName, request.requestID).Metadata(), state.WithEventTypes(state.Destroyed)); err != nil {
			return fmt.Errorf("failed to watch for volume mount status to be destroyed %q: %w", request.requestID, err)
		}
	}

	return nil
}

type volumeMountedCondition struct {
	st       state.State
	id       string
	volumeID string
}

func (cond *volumeMountedCondition) Wait(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_, err := cond.st.WatchFor(ctx, block.NewVolumeMountStatus(block.NamespaceName, cond.id).Metadata(), state.WithEventTypes(state.Created, state.Updated))
	if err != nil {
		return err
	}

	return cond.st.AddFinalizer(ctx, block.NewVolumeMountStatus(block.NamespaceName, cond.id).Metadata(), "service")
}

func (cond *volumeMountedCondition) String() string {
	return fmt.Sprintf("volume %q to be mounted", cond.volumeID)
}

// WaitForVolumeToBeMounted is a service condition that will wait for the volume to be mounted.
func WaitForVolumeToBeMounted(st state.State, requestID, volumeID string) conditions.Condition {
	return &volumeMountedCondition{
		st:       st,
		id:       requestID,
		volumeID: volumeID,
	}
}
