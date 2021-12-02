// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// NodenameReadyCondition implements condition which waits for the nodename to be ready.
type NodenameReadyCondition struct {
	state state.State
}

// NewNodenameReadyCondition builds a coondition which waits for the network to be ready.
func NewNodenameReadyCondition(state state.State) *NodenameReadyCondition {
	return &NodenameReadyCondition{
		state: state,
	}
}

func (condition *NodenameReadyCondition) String() string {
	return "nodename"
}

// Wait implements condition interface.
func (condition *NodenameReadyCondition) Wait(ctx context.Context) error {
	_, err := condition.state.WatchFor(
		ctx,
		resource.NewMetadata(NamespaceName, NodenameType, NodenameID, resource.VersionUndefined),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			if resource.IsTombstone(r) {
				return false, nil
			}

			nodename := r.(*Nodename).TypedSpec()

			// check that hostname status version matches one recorded in the nodename
			hostnameStatus, err := condition.state.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
			if err != nil {
				return false, err
			}

			if hostnameStatus.Metadata().Version().String() != nodename.HostnameVersion {
				return false, nil
			}

			return true, nil
		}),
	)

	return err
}
