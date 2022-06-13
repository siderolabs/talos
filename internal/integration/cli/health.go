// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"fmt"
	"regexp"
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

// TestClientSideWithExplicitNodes does successful health check run from client-side, providing the explicit set of nodes.
//
//nolint:gocyclo
func (suite *HealthSuite) TestClientSideWithExplicitNodes() {
	bootstrapAPIIsUsed := true

	for _, node := range suite.Cluster.Info().Nodes {
		if node.Type == machine.TypeInit {
			bootstrapAPIIsUsed = false
		}
	}

	var args []string

	if bootstrapAPIIsUsed {
		for _, node := range suite.Cluster.Info().Nodes {
			switch node.Type {
			case machine.TypeControlPlane:
				args = append(args, "--control-plane-nodes", node.IPs[0].String())
			case machine.TypeWorker:
				args = append(args, "--worker-nodes", node.IPs[0].String())
				// todo: add more
			case machine.TypeInit, machine.TypeUnknown:
				fallthrough
			default:
				panic(fmt.Sprintf("unexpected machine type %v", node.Type))
			}
		}
	} else {
		for _, node := range suite.Cluster.Info().Nodes {
			switch node.Type {
			case machine.TypeInit:
				args = append(args, "--init-node", node.IPs[0].String())
			case machine.TypeControlPlane:
				args = append(args, "--control-plane-nodes", node.IPs[0].String())
			case machine.TypeWorker:
				args = append(args, "--worker-nodes", node.IPs[0].String())
			case machine.TypeUnknown:
				fallthrough
			default:
				panic(fmt.Sprintf("unexpected machine type %v", node.Type))
			}
		}
	}

	suite.testClientSide(args...)
}

// TestClientSideWithDiscovery does a health check run from client-side without explicitly specifying the nodes.
// It verifies that the check still passes, because the nodes get populated by the discovery service.
//
//nolint:gocyclo
func (suite *HealthSuite) TestClientSideWithDiscovery() {
	suite.testClientSide()
}

// TestServerSide does successful health check run from server-side.
func (suite *HealthSuite) TestServerSide() {
	randomControlPlaneNodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	suite.RunCLI([]string{"health", "--nodes", randomControlPlaneNodeInternalIP},
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`waiting for all k8s nodes to report ready`)),
	)
}

func (suite *HealthSuite) testClientSide(extraArgs ...string) {
	if suite.Cluster == nil {
		suite.T().Skip("Cluster is not available, skipping test")
	}

	args := append([]string{"--server=false"}, extraArgs...)

	if suite.K8sEndpoint != "" {
		args = append(args, "--k8s-endpoint", strings.Split(suite.K8sEndpoint, ":")[0])
	}

	suite.RunCLI(append([]string{"health"}, args...),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`waiting for all k8s nodes to report ready`)),
	)
}

func init() {
	allSuites = append(allSuites, new(HealthSuite))
}
