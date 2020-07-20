// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// ReadSuite verifies dmesg command.
type ReadSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ReadSuite) SuiteName() string {
	return "cli.ReadSuite"
}

// TestSuccess runs comand with success.
func (suite *ReadSuite) TestSuccess() {
	suite.RunCLI([]string{"read", "--nodes", suite.RandomDiscoveredNode(), "/etc/os-release"},
		base.StdoutShouldMatch(regexp.MustCompile(`ID=talos`)))
}

// TestMultiNodeFail verifies that command fails with multiple nodes.
func (suite *ReadSuite) TestMultiNodeFail() {
	suite.RunCLI([]string{"read", "--nodes", "127.0.0.1", "--nodes", "127.0.0.1", "/etc/os-release"},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`is not supported with multiple nodes`)))
}

func init() {
	allSuites = append(allSuites, new(ReadSuite))
}
