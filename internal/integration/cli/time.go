// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"regexp"
	"time"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/integration/base"
)

// TimeSuite verifies dmesg command.
type TimeSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *TimeSuite) SuiteName() string {
	return "cli.TimeSuite"
}

// TestDefault runs default time check.
func (suite *TimeSuite) TestDefault() {
	suite.RunCLI([]string{"time", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`NTP-SERVER`)),
		base.StdoutShouldMatch(regexp.MustCompile(`UTC`)),
		base.WithRetry(retry.Constant(time.Minute, retry.WithUnits(time.Second))),
	)
}

func init() {
	allSuites = append(allSuites, new(TimeSuite))
}
