// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/integration/base"
)

// CrashdumpSuite verifies crashdump command
type CrashdumpSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *CrashdumpSuite) SuiteName() string {
	return "cli.CrashdumpSuite"
}

// TestRun does successful health check run.
func (suite *CrashdumpSuite) TestRun() {
	if suite.Cluster == nil {
		suite.T().Skip("Cluster is not available, skipping test")
	}

	args := []string{}
	for _, node := range suite.Cluster.Info().Nodes {
		switch node.Type {
		case runtime.MachineTypeInit:
			args = append(args, "--init-node", node.PrivateIP.String())
		case runtime.MachineTypeControlPlane:
			args = append(args, "--control-plane-nodes", node.PrivateIP.String())
		case runtime.MachineTypeJoin:
			args = append(args, "--worker-nodes", node.PrivateIP.String())
		}
	}

	suite.RunCLI(append([]string{"crashdump"}, args...),
		base.StdoutShouldMatch(regexp.MustCompile(`> containerd`)),
	)
}

func init() {
	allSuites = append(allSuites, new(CrashdumpSuite))
}
