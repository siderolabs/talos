// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

// ServiceStateAssertion checks whether service reached some specified state.
func ServiceStateAssertion(ctx context.Context, cluster provision.ClusterAccess, service, state string) error {
	client, err := cluster.Client()
	if err != nil {
		return err
	}

	servicesInfo, err := client.ServiceInfo(ctx, service)
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
