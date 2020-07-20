// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// ProcessesSuite verifies dmesg command.
type ProcessesSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ProcessesSuite) SuiteName() string {
	return "cli.ProcessesSuite"
}

// TestSuccess verifies successful execution.
func (suite *ProcessesSuite) TestSuccess() {
	suite.RunCLI([]string{"processes", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`PID`)))
}

func init() {
	allSuites = append(allSuites, new(ProcessesSuite))
}
