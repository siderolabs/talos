// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/pkg/machinery/client"
)

// ApidReadyAssertion checks whether apid is responsive on all the nodes.
func ApidReadyAssertion(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("failed to create cluster client: %w", err)
	}

	nodesCtx := client.WithNodes(ctx, cluster.Nodes()...)

	_, err = cli.Version(nodesCtx)

	return fmt.Errorf("failed to get version: %w", err)
}
