// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

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

	type serviceInfoWithNode struct {
		client.ServiceInfo

		node string
	}

	var servicesInfo []serviceInfoWithNode

	respCh := multiplex.Unary(
		ctx, mapIPsToStrings(mapNodeInfosToInternalIPs(nodes)),
		func(ctx context.Context) ([]client.ServiceInfo, error) {
			return cli.ServiceInfo(ctx, service)
		},
	)

	for resp := range respCh {
		if resp.Err != nil {
			return fmt.Errorf("error getting service info for service %q from node %q: %w", service, resp.Node, resp.Err)
		}

		servicesInfo = append(
			servicesInfo,
			xslices.Map(
				resp.Payload,
				func(info client.ServiceInfo) serviceInfoWithNode {
					return serviceInfoWithNode{
						ServiceInfo: info,
						node:        resp.Node,
					}
				},
			)...,
		)
	}

	if len(servicesInfo) != count {
		return fmt.Errorf("expected a response with %d node(s), got %d", count, len(servicesInfo))
	}

	var multiErr *multierror.Error

	// sort service info list so that errors returned are consistent
	slices.SortFunc(servicesInfo, func(a, b serviceInfoWithNode) int {
		return cmp.Compare(a.node, b.node)
	})

	for _, serviceInfo := range servicesInfo {
		node := serviceInfo.node

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
