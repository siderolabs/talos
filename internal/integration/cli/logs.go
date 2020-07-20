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

// LogsSuite verifies logs command.
type LogsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *LogsSuite) SuiteName() string {
	return "cli.LogsSuite"
}

// TestServiceLogs verifies that logs are displayed.
func (suite *LogsSuite) TestServiceLogs() {
	suite.RunCLI([]string{"logs", "kubelet", "--nodes", suite.RandomDiscoveredNode()}) // default checks for stdout not empty
}

// TestTailLogs verifies that logs can be displayed with tail lines.
func (suite *LogsSuite) TestTailLogs() {
	node := suite.RandomDiscoveredNode()

	// run some machined API calls to produce enough log lines
	for i := 0; i < 10; i++ {
		suite.RunCLI([]string{"-n", node, "version"})
	}

	suite.RunCLI([]string{"logs", "apid", "-n", node, "--tail", "5"},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Count(stdout, "\n")
			if lines != 5 {
				return fmt.Errorf("expected %d lines, found %d lines", 5, lines)
			}

			return nil
		}))
}

// TestServiceNotFound verifies that logs displays an error if service is not found.
func (suite *LogsSuite) TestServiceNotFound() {
	suite.RunCLI([]string{"logs", "--nodes", suite.RandomDiscoveredNode(), "servicenotfound"},
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`ERROR:.+ log "servicenotfound" was not registered`)),
	)
}

func init() {
	allSuites = append(allSuites, new(LogsSuite))
}
