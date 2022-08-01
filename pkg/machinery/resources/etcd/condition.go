// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// SpecReadyCondition implements condition which waits for the etcd spec to be ready.
type SpecReadyCondition struct {
	state state.State
}

// NewSpecReadyCondition builds a condition which waits for the etcd spec to be ready.
func NewSpecReadyCondition(state state.State) *SpecReadyCondition {
	return &SpecReadyCondition{
		state: state,
	}
}

func (condition *SpecReadyCondition) String() string {
	return "etcd spec"
}

// Wait implements condition interface.
func (condition *SpecReadyCondition) Wait(ctx context.Context) error {
	_, err := condition.state.WatchFor(
		ctx,
		resource.NewMetadata(NamespaceName, SpecType, SpecID, resource.VersionUndefined),
		state.WithEventTypes(state.Created, state.Updated),
	)

	return err
}
