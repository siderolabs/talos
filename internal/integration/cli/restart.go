// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"math/rand"
	"regexp"
	"testing"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
)

// RestartSuite verifies dmesg command.
type RestartSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *RestartSuite) SuiteName() string {
	return "cli.RestartSuite"
}

// TestSystem restarts system containerd process.
func (suite *RestartSuite) TestSystem() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	nodes := suite.DiscoverNodes()
	suite.Require().NotEmpty(nodes)

	// trustd only runs on control plane nodes
	node := nodes[0]

	suite.RunCLI([]string{"restart", "-n", node, "trustd"},
		base.StdoutEmpty())

	time.Sleep(200 * time.Millisecond)

	suite.RunAndWaitForMatch([]string{"service", "-n", node, "trustd"}, regexp.MustCompile(`EVENTS\s+\[Running\]: Health check successful`), 30*time.Second)
}

// TestKubernetes restarts K8s container.
func (suite *RestartSuite) TestK8s() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	nodes := suite.DiscoverNodes()
	suite.Require().NotEmpty(nodes)

	node := nodes[rand.Intn(len(nodes))]

	suite.RunCLI([]string{"restart", "-n", node, "-k", "kubelet"},
		base.StdoutEmpty())

	time.Sleep(200 * time.Millisecond)

	suite.RunAndWaitForMatch([]string{"service", "-n", node, "kubelet"}, regexp.MustCompile(`EVENTS\s+\[Running\]: Health check successful`), 30*time.Second)
}

func init() {
	allSuites = append(allSuites, new(RestartSuite))
}
