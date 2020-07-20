// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/talos-systems/talos/internal/integration/base"
)

// DmesgSuite verifies dmesg command.
type DmesgSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *DmesgSuite) SuiteName() string {
	return "cli.DmesgSuite"
}

// TestHasOutput verifies that dmesg is displayed.
func (suite *DmesgSuite) TestHasOutput() {
	suite.RunCLI([]string{"dmesg", "--nodes", suite.RandomDiscoveredNode()}) // default checks for stdout not empty
}

// TestClusterHasOutput verifies that each node in the cluster has some output.
func (suite *DmesgSuite) TestClusterHasOutput() {
	nodes := suite.DiscoverNodes().Nodes()
	suite.Require().NotEmpty(nodes)

	matchers := make([]base.RunOption, 0, len(nodes))

	for _, node := range nodes {
		matchers = append(matchers,
			base.StdoutShouldMatch(
				regexp.MustCompile(fmt.Sprintf(`(?m)^%s:`, regexp.QuoteMeta(node)))))
	}

	suite.RunCLI([]string{"--nodes", strings.Join(nodes, ","), "dmesg"},
		matchers...)
}

func init() {
	allSuites = append(allSuites, new(DmesgSuite))
}
