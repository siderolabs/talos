// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// ResourceLister is the subset of the resource state required to discover the etcd endpoints.
//
// It is implemented both by the resource state, and by the controller runtime.
type ResourceLister interface {
	List(context.Context, resource.Kind, ...state.ListOption) (resource.List, error)
}

// errNoEndpoints is returned when the control plane machines are not discovered yet.
var errNoEndpoints = errors.New("no controlplane endpoints discovered yet")

// LocalEndpoints returns the endpoints of the etcd member running on this machine.
func LocalEndpoints() []string {
	return []string{nethelpers.JoinHostPort("localhost", constants.EtcdClientPort)}
}

// GetEndpoints returns expected endpoints of etcd cluster members.
//
// It is not guaranteed that etcd is running on each listed endpoint.
func GetEndpoints(ctx context.Context, resources ResourceLister) ([]string, error) {
	endpoints, err := ListEndpoints(ctx, resources)
	if err != nil {
		return nil, err
	}

	if len(endpoints) == 0 {
		return nil, errNoEndpoints
	}

	return endpoints, nil
}

// ListEndpoints returns expected endpoints of etcd cluster members, the list is empty
// if the control plane machines are not discovered yet.
//
// It is not guaranteed that etcd is running on each listed endpoint.
func ListEndpoints(ctx context.Context, resources ResourceLister) ([]string, error) {
	endpointResources, err := resources.List(ctx, k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "").Metadata())
	if err != nil {
		return nil, fmt.Errorf("error getting endpoints resources: %w", err)
	}

	var endpointAddrs k8s.EndpointList

	// merge all endpoints into a single list
	for _, res := range endpointResources.Items {
		if endpoint, ok := res.(*k8s.Endpoint); ok {
			endpointAddrs = endpointAddrs.Merge(endpoint)
		}
	}

	if endpointAddrs.IsEmpty() {
		return nil, nil
	}

	endpoints := endpointAddrs.Strings()

	// Etcd expects host:port format.
	for i := range endpoints {
		endpoints[i] = nethelpers.JoinHostPort(endpoints[i], constants.EtcdClientPort)
	}

	return endpoints, nil
}
