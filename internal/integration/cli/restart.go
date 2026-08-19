// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"
	"testing"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
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
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.RunCLI([]string{"restart", "-n", node, "trustd"},
		base.StdoutEmpty())

	time.Sleep(200 * time.Millisecond)

	suite.RunAndWaitForMatch([]string{"service", "-n", node, "trustd"}, regexp.MustCompile(`EVENTS\s+\[Running\]: Health check successful`), 30*time.Second)
}

// TestKubernetesFlagDeprecated covers the deprecated -k/--kubernetes alias reaching the CRI driver, and
// warning while it does.
//
// A nonexistent container id avoids restarting a real Kubernetes workload container, while still
// proving the request reached the CRI driver via its "not found" response.
func (suite *RestartSuite) TestKubernetesFlagDeprecated() {
	suite.RunCLI(
		[]string{
			"restart", "-k", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane),
			"talosctl-it-nonexistent",
		},
		base.StdoutEmpty(),
		base.ShouldFail(),
		base.StderrShouldMatch(regexp.MustCompile(`(?i)deprecated`)),
		base.StderrShouldMatch(regexp.MustCompile(`--namespace cri`)),
		base.StderrShouldMatch(regexp.MustCompile(`not found`)),
	)
}

func init() {
	allSuites = append(allSuites, new(RestartSuite))
}
