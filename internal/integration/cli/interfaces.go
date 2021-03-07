// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// InterfacesSuite verifies dmesg command.
type InterfacesSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *InterfacesSuite) SuiteName() string {
	return "cli.InterfacesSuite"
}

// TestSuccess verifies successful execution.
func (suite *InterfacesSuite) TestSuccess() {
	suite.RunCLI([]string{"interfaces", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`lo`)))
}

func init() {
	allSuites = append(allSuites, new(InterfacesSuite))
}
