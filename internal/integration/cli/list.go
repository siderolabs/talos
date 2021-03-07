// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// ListSuite verifies dmesg command.
type ListSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ListSuite) SuiteName() string {
	return "cli.ListSuite"
}

// TestSuccess runs comand with success.
func (suite *ListSuite) TestSuccess() {
	suite.RunCLI([]string{"list", "--nodes", suite.RandomDiscoveredNode(), "/etc"},
		base.StdoutShouldMatch(regexp.MustCompile(`os-release`)))

	suite.RunCLI([]string{"list", "--nodes", suite.RandomDiscoveredNode(), "/"},
		base.StdoutShouldNotMatch(regexp.MustCompile(`os-release`)))
}

func init() {
	allSuites = append(allSuites, new(ListSuite))
}
