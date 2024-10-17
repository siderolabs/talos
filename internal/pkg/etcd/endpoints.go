// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// GetEndpoints returns expected endpoints of etcd cluster members.
//
// It is not guaranteed that etcd is running on each listed endpoint.
func GetEndpoints(ctx context.Context, resources state.State) ([]string, error) {
	endpointResources, err := safe.StateListAll[*k8s.Endpoint](ctx, resources)
	if err != nil {
		return nil, fmt.Errorf("error getting endpoints resources: %w", err)
	}

	var endpointAddrs k8s.EndpointList

	// merge all endpoints into a single list
	for res := range endpointResources.All() {
		endpointAddrs = endpointAddrs.Merge(res)
	}

	if len(endpointAddrs) == 0 {
		return nil, errors.New("no controlplane endpoints discovered yet")
	}

	endpoints := endpointAddrs.Strings()

	// Etcd expects host:port format.
	for i := range endpoints {
		endpoints[i] = nethelpers.JoinHostPort(endpoints[i], constants.EtcdClientPort)
	}

	return endpoints, nil
}
