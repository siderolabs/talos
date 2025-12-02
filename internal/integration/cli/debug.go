// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"
	"strings"

	"github.com/siderolabs/talos/internal/integration/base"
)

// DebugSuite verifies debug command.
type DebugSuite struct {
	base.CLISuite
}

const debugImage = "docker.io/library/alpine:3.23"

// SuiteName ...
func (suite *DebugSuite) SuiteName() string {
	return "cli.DebugSuite"
}

// TestSuccess runs comand with success.
func (suite *DebugSuite) TestSuccess() {
	suite.RunCLI([]string{"debug", debugImage, "--nodes", suite.RandomDiscoveredNodeInternalIP()},
		base.StdoutShouldMatch(regexp.MustCompile("Linux")),
		base.StderrNotEmpty(),
		base.WithStdin(strings.NewReader("uname\n")),
	)
}

func init() {
	allSuites = append(allSuites, new(DebugSuite))
}
