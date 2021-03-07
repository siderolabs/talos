// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// VersionSuite verifies version command.
type VersionSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *VersionSuite) SuiteName() string {
	return "cli.VersionSuite"
}

// TestExpectedVersionMaster verifies master node version matches expected.
func (suite *VersionSuite) TestExpectedVersionMaster() {
	suite.RunCLI([]string{"version", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`Client:\n\s*Tag:\s*`+regexp.QuoteMeta(suite.Version))),
		base.StdoutShouldMatch(regexp.MustCompile(`Server:\n(\s*NODE:[^\n]+\n)?\s*Tag:\s*`+regexp.QuoteMeta(suite.Version))),
	)
}

// TestShortVersion verifies short version output.
func (suite *VersionSuite) TestShortVersion() {
	suite.RunCLI([]string{"version", "--short", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`Client\s*`+regexp.QuoteMeta(suite.Version))),
	)
}

// TestClient verifies only client version output.
func (suite *VersionSuite) TestClient() {
	suite.RunCLI([]string{"version", "--client"},
		base.StdoutShouldMatch(regexp.MustCompile(`Client:\n\s*Tag:\s*`+regexp.QuoteMeta(suite.Version))),
		base.StdoutShouldNotMatch(regexp.MustCompile(`Server`)),
	)
}

func init() {
	allSuites = append(allSuites, new(VersionSuite))
}
