// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// RoutesSuite verifies dmesg command.
type RoutesSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *RoutesSuite) SuiteName() string {
	return "cli.RoutesSuite"
}

// TestSuccess verifies successful execution.
func (suite *RoutesSuite) TestSuccess() {
	suite.RunCLI([]string{"routes", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`GATEWAY`)),
		base.StdoutShouldMatch(regexp.MustCompile(`127\.0\.0\.0/8`)),
	)
}

func init() {
	allSuites = append(allSuites, new(RoutesSuite))
}
