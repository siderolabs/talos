// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

// ApidReadyAssertion checks whether apid is responsive on all the nodes.
func ApidReadyAssertion(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	respCh := multiplex.Unary(
		ctx, mapIPsToStrings(mapNodeInfosToInternalIPs(cluster.Nodes())),
		func(ctx context.Context) (*machine.VersionResponse, error) {
			return cli.Version(ctx)
		},
	)

	var errs error

	for resp := range respCh {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error getting version from node %q: %w", resp.Node, resp.Err))
		}
	}

	return errs
}
