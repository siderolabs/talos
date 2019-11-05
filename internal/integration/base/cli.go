// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package base

import (
	"os/exec"

	"github.com/stretchr/testify/suite"
)

// CLISuite is a base suite for CLI tests
type CLISuite struct {
	suite.Suite
	TalosSuite
}

// RunOsctl runs osctl binary with the options provided
func (cliSuite *CLISuite) RunOsctl(args []string, options ...RunOption) {
	if cliSuite.Target != "" {
		args = append([]string{"--target", cliSuite.Target}, args...)
	}

	args = append([]string{"--talosconfig", cliSuite.TalosConfig}, args...)

	cmd := exec.Command(cliSuite.OsctlPath, args...)

	Run(&cliSuite.Suite, cmd, options...)
}
