// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/internal/pkg/provision"
)

// ServiceStateAssertion checks whether service reached some specified state.
func ServiceStateAssertion(ctx context.Context, cluster provision.ClusterAccess, service, state string) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	servicesInfo, err := cli.ServiceInfo(ctx, service)
	if err != nil {
		return err
	}

	serviceOk := false

	for _, serviceInfo := range servicesInfo {
		if len(serviceInfo.Service.Events.Events) == 0 {
			return fmt.Errorf("no events recorded yet for service %q", service)
		}

		lastEvent := serviceInfo.Service.Events.Events[len(serviceInfo.Service.Events.Events)-1]
		if lastEvent.State != state {
			return fmt.Errorf("service %q not in expected state %q: current state [%s] %s", service, state, lastEvent.State, lastEvent.Msg)
		}

		serviceOk = true
	}

	if !serviceOk {
		return fmt.Errorf("service %q not found", service)
	}

	return nil
}

// ServiceHealthAssertion checks whether service reached some specified state.
//nolint: gocyclo
func ServiceHealthAssertion(ctx context.Context, cluster provision.ClusterAccess, service string, setters ...Option) error {
	opts := DefaultOptions()

	for _, setter := range setters {
		if err := setter(opts); err != nil {
			return err
		}
	}

	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	nodes := make([]string, 0, len(cluster.Info().Nodes))

	for _, node := range cluster.Info().Nodes {
		if len(opts.Types) > 0 {
			for _, t := range opts.Types {
				if node.Type == t {
					nodes = append(nodes, node.PrivateIP.String())
				}
			}

			continue
		}

		nodes = append(nodes, node.PrivateIP.String())
	}

	count := len(nodes)

	nodesCtx := client.WithNodes(ctx, nodes...)

	servicesInfo, err := cli.ServiceInfo(nodesCtx, service)
	if err != nil {
		return err
	}

	if len(servicesInfo) != count {
		return fmt.Errorf("expected a response with %d node(s), got %d", count, len(servicesInfo))
	}

	for _, serviceInfo := range servicesInfo {
		if len(serviceInfo.Service.Events.Events) == 0 {
			return fmt.Errorf("no events recorded yet for service %q", service)
		}

		lastEvent := serviceInfo.Service.Events.Events[len(serviceInfo.Service.Events.Events)-1]
		if lastEvent.State != "Running" {
			return fmt.Errorf("service %q not in expected state %q: current state [%s] %s", service, "Running", lastEvent.State, lastEvent.Msg)
		}

		if !serviceInfo.Service.GetHealth().GetHealthy() {
			return fmt.Errorf("service is not healthy: %s", service)
		}
	}

	return nil
}
