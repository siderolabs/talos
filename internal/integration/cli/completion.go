// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"github.com/talos-systems/talos/internal/integration/base"
)

// CompletionSuite verifies dmesg command.
type CompletionSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *CompletionSuite) SuiteName() string {
	return "cli.CompletionSuite"
}

// TestSuccess runs comand with success.
func (suite *CompletionSuite) TestSuccess() {
	suite.RunCLI([]string{"completion", "bash"})
	suite.RunCLI([]string{"completion", "fish"})
	suite.RunCLI([]string{"completion", "zsh"})
}

func init() {
	allSuites = append(allSuites, new(CompletionSuite))
}
