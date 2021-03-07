// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"
	"testing"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
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

	// trustd only runs on control plane nodes
	node := suite.RandomDiscoveredNode(machine.TypeControlPlane)

	suite.RunCLI([]string{"restart", "-n", node, "trustd"},
		base.StdoutEmpty())

	time.Sleep(200 * time.Millisecond)

	suite.RunAndWaitForMatch([]string{"service", "-n", node, "trustd"}, regexp.MustCompile(`EVENTS\s+\[Running\]: Health check successful`), 30*time.Second)
}

func init() {
	allSuites = append(allSuites, new(RestartSuite))
}
