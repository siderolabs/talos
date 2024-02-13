// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// ErrServiceNotFound is an error that indicates that a service was not found.
var ErrServiceNotFound = errors.New("service not found")

// ServiceStateAssertion checks whether service reached some specified state.
func ServiceStateAssertion(ctx context.Context, cl ClusterInfo, service string, states ...string) error {
	cli, err := cl.Client()
	if err != nil {
		return err
	}

	// by default, we check all control plane nodes. if some nodes don't have that service running,
	// it won't be returned in the response
	nodes := append(cl.NodesByType(machine.TypeInit), cl.NodesByType(machine.TypeControlPlane)...)

	nodesCtx := client.WithNodes(ctx, mapIPsToStrings(mapNodeInfosToInternalIPs(nodes))...)

	servicesInfo, err := cli.ServiceInfo(nodesCtx, service)
	if err != nil {
		return err
	}

	if len(servicesInfo) == 0 {
		return ErrServiceNotFound
	}

	acceptedStates := map[string]struct{}{}
	for _, state := range states {
		acceptedStates[state] = struct{}{}
	}

	var multiErr *multierror.Error

	for _, serviceInfo := range servicesInfo {
		node := serviceInfo.Metadata.GetHostname()

		if len(serviceInfo.Service.Events.Events) == 0 {
			multiErr = multierror.Append(multiErr, fmt.Errorf("%s: no events recorded yet for service %q", node, service))

			continue
		}

		lastEvent := serviceInfo.Service.Events.Events[len(serviceInfo.Service.Events.Events)-1]
		if _, ok := acceptedStates[lastEvent.State]; !ok {
			multiErr = multierror.Append(multiErr, fmt.Errorf("%s: service %q not in expected state %q: current state [%s] %s", node, service, states, lastEvent.State, lastEvent.Msg))
		}
	}

	return multiErr.ErrorOrNil()
}

// ServiceHealthAssertion checks whether service reached some specified state.
//
//nolint:gocyclo
func ServiceHealthAssertion(ctx context.Context, cl ClusterInfo, service string, setters ...Option) error {
	opts := DefaultOptions()

	for _, setter := range setters {
		if err := setter(opts); err != nil {
			return err
		}
	}

	cli, err := cl.Client()
	if err != nil {
		return err
	}

	var nodes []cluster.NodeInfo

	if len(opts.Types) > 0 {
		for _, t := range opts.Types {
			nodes = append(nodes, cl.NodesByType(t)...)
		}
	} else {
		nodes = cl.Nodes()
	}

	count := len(nodes)

	nodesCtx := client.WithNodes(ctx, mapIPsToStrings(mapNodeInfosToInternalIPs(nodes))...)

	servicesInfo, err := cli.ServiceInfo(nodesCtx, service)
	if err != nil {
		return err
	}

	if len(servicesInfo) != count {
		return fmt.Errorf("expected a response with %d node(s), got %d", count, len(servicesInfo))
	}

	var multiErr *multierror.Error

	// sort service info list so that errors returned are consistent
	sort.Slice(servicesInfo, func(i, j int) bool {
		return servicesInfo[i].Metadata.GetHostname() < servicesInfo[j].Metadata.GetHostname()
	})

	for _, serviceInfo := range servicesInfo {
		node := serviceInfo.Metadata.GetHostname()

		if len(serviceInfo.Service.Events.Events) == 0 {
			multiErr = multierror.Append(multiErr, fmt.Errorf("%s: no events recorded yet for service %q", node, service))

			continue
		}

		lastEvent := serviceInfo.Service.Events.Events[len(serviceInfo.Service.Events.Events)-1]
		if lastEvent.State != "Running" {
			multiErr = multierror.Append(multiErr, fmt.Errorf("%s: service %q not in expected state %q: current state [%s] %s", node, service, "Running", lastEvent.State, lastEvent.Msg))

			continue
		}

		if !serviceInfo.Service.GetHealth().GetHealthy() {
			multiErr = multierror.Append(multiErr, fmt.Errorf("%s: service is not healthy: %s", node, service))

			continue
		}
	}

	return multiErr.ErrorOrNil()
}
