// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// NoDiagnostics checks whether there are no diagnostic warnings.
func NoDiagnostics(ctx context.Context, cluster ClusterInfo) error {
	cli, err := cluster.Client()
	if err != nil {
		return err
	}

	nodes := cluster.Nodes()
	nodeInternalIPs := mapIPsToStrings(mapNodeInfosToInternalIPs(nodes))

	warningsByNode := map[string][]*runtime.Diagnostic{}

	for _, nodeIP := range nodeInternalIPs {
		warnings, err := safe.StateListAll[*runtime.Diagnostic](client.WithNode(ctx, nodeIP), cli.COSI)
		if err != nil {
			if client.StatusCode(err) == codes.PermissionDenied {
				// not supported, skip
				return conditions.ErrSkipAssertion
			}

			return err
		}

		for res := range warnings.All() {
			warningsByNode[nodeIP] = append(warningsByNode[nodeIP], res)
		}
	}

	if len(warningsByNode) == 0 {
		return nil
	}

	nodesWithWarnings := maps.Keys(warningsByNode)
	slices.Sort(nodesWithWarnings)

	return fmt.Errorf("active diagnostics: %s", strings.Join(xslices.Map(nodesWithWarnings, func(node string) string {
		return node + ": " + strings.Join(xslices.Map(warningsByNode[node], func(warning *runtime.Diagnostic) string {
			return warning.TypedSpec().Message
		}), ", ")
	}), "; "))
}
