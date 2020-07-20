// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// MemorySuite verifies dmesg command.
type MemorySuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *MemorySuite) SuiteName() string {
	return "cli.MemorySuite"
}

// TestSuccess verifies successful execution.
func (suite *MemorySuite) TestSuccess() {
	suite.RunCLI([]string{"memory", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`FREE`)))
}

// TestVerbose verifies verbose mode.
func (suite *MemorySuite) TestVerbose() {
	suite.RunCLI([]string{"memory", "-v", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`MemFree: \d+ kB`)))
}

func init() {
	allSuites = append(allSuites, new(MemorySuite))
}
