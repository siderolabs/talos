// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// ServicesSuite verifies dmesg command.
type ServicesSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ServicesSuite) SuiteName() string {
	return "cli.ServicesSuite"
}

// TestList verifies service list.
func (suite *ServicesSuite) TestList() {
	suite.RunCLI([]string{"services", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`STATE`)),
		base.StdoutShouldMatch(regexp.MustCompile(`apid`)),
	)
}

// TestStatus verifies service status.
func (suite *ServicesSuite) TestStatus() {
	suite.RunCLI([]string{"service", "apid", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`STATE`)),
		base.StdoutShouldMatch(regexp.MustCompile(`apid`)),
		base.StdoutShouldMatch(regexp.MustCompile(`\[Running\]`)),
	)
}

func init() {
	allSuites = append(allSuites, new(ServicesSuite))
}
