// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// VersionSuite verifies version API
type VersionSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *VersionSuite) SuiteName() string {
	return "cli.VersionSuite"
}

// TestExpectedVersionMaster verifies master node version matches expected
func (suite *VersionSuite) TestExpectedVersionMaster() {
	suite.RunOsctl([]string{"version"},
		base.StdoutShouldMatch(regexp.MustCompile(`Client:\n\s*Tag:\s*`+regexp.QuoteMeta(suite.Version))),
		base.StdoutShouldMatch(regexp.MustCompile(`Server:\n(\s*NODE:[^\n]+\n)?\s*Tag:\s*`+regexp.QuoteMeta(suite.Version))),
	)
}

func init() {
	allSuites = append(allSuites, new(VersionSuite))
}
