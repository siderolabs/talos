// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// LogsSuite verifies logs command
type LogsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *LogsSuite) SuiteName() string {
	return "cli.LogsSuite"
}

// TestServiceLogs verifies that logs are displayed.
func (suite *LogsSuite) TestServiceLogs() {
	suite.RunOsctl([]string{"logs", "kubelet"}) // default checks for stdout not empty
}

// TestServiceNotFound verifies that logs displays an error if service is not found.
func (suite *LogsSuite) TestServiceNotFound() {
	suite.RunOsctl([]string{"logs", "servicenotfound"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile("error getting logs: .*servicenotfound.log: no such file or directory")),
	)
}

func init() {
	allSuites = append(allSuites, new(LogsSuite))
}
