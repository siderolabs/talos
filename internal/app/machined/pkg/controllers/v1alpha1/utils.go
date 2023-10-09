// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// WaitForServiceHealthy waits for a service to be healthy.
//
// It is a helper function for controllers.
func WaitForServiceHealthy(ctx context.Context, r controller.Runtime, serviceID string, nextInputs []controller.Input) error {
	// set inputs temporarily to a service only
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(serviceID),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.EventCh():
		}

		service, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, serviceID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if service.TypedSpec().Running && service.TypedSpec().Healthy {
			// condition met
			break
		}
	}

	// restore inputs
	if err := r.UpdateInputs(nextInputs); err != nil {
		return err
	}

	// queue an update to reprocess with new inputs
	r.QueueReconcile()

	return nil
}
