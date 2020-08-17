// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"
	"sort"
	"strings"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// HealthSuite verifies health command.
type HealthSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *HealthSuite) SuiteName() string {
	return "cli.HealthSuite"
}

// TestClientSide does successful health check run from client-side.
func (suite *HealthSuite) TestClientSide() {
	if suite.Cluster == nil {
		suite.T().Skip("Cluster is not available, skipping test")
	}

	args := []string{"--server=false"}

	bootstrapAPIIsUsed := true

	for _, node := range suite.Cluster.Info().Nodes {
		if node.Type == machine.TypeInit {
			bootstrapAPIIsUsed = false
		}
	}

	if bootstrapAPIIsUsed {
		nodes := []string{}

		for _, node := range suite.Cluster.Info().Nodes {
			switch node.Type {
			case machine.TypeControlPlane:
				nodes = append(nodes, node.PrivateIP.String())
			case machine.TypeJoin:
				args = append(args, "--worker-nodes", node.PrivateIP.String())
			}
		}

		sort.Strings(nodes)

		if len(nodes) > 0 {
			args = append(args, "--init-node", nodes[0])
		}

		if len(nodes) > 1 {
			args = append(args, "--control-plane-nodes", strings.Join(nodes[1:], ","))
		}
	} else {
		for _, node := range suite.Cluster.Info().Nodes {
			switch node.Type {
			case machine.TypeInit:
				args = append(args, "--init-node", node.PrivateIP.String())
			case machine.TypeControlPlane:
				args = append(args, "--control-plane-nodes", node.PrivateIP.String())
			case machine.TypeJoin:
				args = append(args, "--worker-nodes", node.PrivateIP.String())
			}
		}
	}

	if suite.K8sEndpoint != "" {
		args = append(args, "--k8s-endpoint", strings.Split(suite.K8sEndpoint, ":")[0])
	}

	suite.RunCLI(append([]string{"health"}, args...),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`waiting for all k8s nodes to report ready`)),
	)
}

// TestServerSide does successful health check run from server-side.
func (suite *HealthSuite) TestServerSide() {
	suite.RunCLI([]string{"health", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane)},
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`waiting for all k8s nodes to report ready`)),
	)
}

func init() {
	allSuites = append(allSuites, new(HealthSuite))
}
