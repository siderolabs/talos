// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// CrashdumpSuite verifies crashdump command.
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
		case machine.TypeInit:
			args = append(args, "--init-node", node.IPs[0].String())
		case machine.TypeControlPlane:
			args = append(args, "--control-plane-nodes", node.IPs[0].String())
		case machine.TypeJoin:
			args = append(args, "--worker-nodes", node.IPs[0].String())
		case machine.TypeUnknown:
			panic("unexpected")
		}
	}

	suite.RunCLI(append([]string{"crashdump"}, args...),
		base.StdoutShouldMatch(regexp.MustCompile(`> containerd`)),
	)
}

func init() {
	allSuites = append(allSuites, new(CrashdumpSuite))
}
