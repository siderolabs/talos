// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// SyncCondition implements condition which waits for the time to be in sync.
type SyncCondition struct {
	state state.State
}

// NewSyncCondition builds a coondition which waits for the time to be in sync.
func NewSyncCondition(state state.State) *SyncCondition {
	return &SyncCondition{
		state: state,
	}
}

func (condition *SyncCondition) String() string {
	return "time sync"
}

// Wait implements condition interface.
func (condition *SyncCondition) Wait(ctx context.Context) error {
	_, err := condition.state.WatchFor(
		ctx,
		resource.NewMetadata(v1alpha1.NamespaceName, StatusType, StatusID, resource.VersionUndefined),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			return r.(*Status).Status().Synced, nil
		}),
	)

	return err
}
