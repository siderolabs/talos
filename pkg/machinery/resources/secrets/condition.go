// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// APIReadyCondition implements condition which waits for the API certs to be ready.
type APIReadyCondition struct {
	state state.State
}

// NewAPIReadyCondition builds a coondition which waits for the API certs to be ready.
func NewAPIReadyCondition(state state.State) *APIReadyCondition {
	return &APIReadyCondition{
		state: state,
	}
}

func (condition *APIReadyCondition) String() string {
	return "api certificates"
}

// Wait implements condition interface.
func (condition *APIReadyCondition) Wait(ctx context.Context) error {
	_, err := condition.state.WatchFor(
		ctx,
		resource.NewMetadata(NamespaceName, APIType, APIID, resource.VersionUndefined),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			if resource.IsTombstone(r) {
				return false, nil
			}

			return true, nil
		}),
	)

	return err
}
