// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"context"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
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
func (suite *HealthSuite) TestClientSideWithExplicitNodes() {
	info := suite.DiscoverNodes(context.TODO())

	var args []string

	for _, machineType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
		for _, node := range info.NodesByType(machineType) {
			switch machineType {
			case machine.TypeInit:
				args = append(args, "--init-node", node.IPs[0].String())
			case machine.TypeControlPlane:
				args = append(args, "--control-plane-nodes", node.IPs[0].String())
			case machine.TypeWorker:
				args = append(args, "--worker-nodes", node.IPs[0].String())
			case machine.TypeUnknown:
				// skip it
			default:
				panic(fmt.Sprintf("unexpected machine type: %v", machineType))
			}
		}
	}

	suite.testClientSide(args...)
}

// TestClientSideWithDiscovery does a health check run from client-side without explicitly specifying the nodes.
// It verifies that the check still passes, because the nodes get populated by the discovery service.
func (suite *HealthSuite) TestClientSideWithDiscovery() {
	discoveryEnabled, err := suite.isDiscoveryEnabled()
	suite.Require().NoError(err)

	if !discoveryEnabled {
		suite.T().Skipf("skipping test: discovery is not enabled on the cluster")
	}

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
	args := append([]string{"--server=false"}, extraArgs...)

	if suite.K8sEndpoint != "" {
		args = append(args, "--k8s-endpoint", suite.K8sEndpoint)
	}

	suite.RunCLI(append([]string{"health"}, args...),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`waiting for all k8s nodes to report ready`)),
	)
}

func (suite *HealthSuite) isDiscoveryEnabled() (bool, error) {
	temp := struct {
		Spec cluster.ConfigSpec `yaml:"spec"`
	}{}

	randomControlPlaneNodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	stdout, _ := suite.RunCLI([]string{"--nodes", randomControlPlaneNodeInternalIP, "get", "discoveryconfigs", "-oyaml"})

	err := yaml.Unmarshal([]byte(stdout), &temp)
	if err != nil {
		return false, err
	}

	return temp.Spec.DiscoveryEnabled, nil
}

func init() {
	allSuites = append(allSuites, new(HealthSuite))
}
