// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"fmt"
	"os"

	"github.com/siderolabs/talos/internal/integration/base"
)

// ValidateSuite verifies dmesg command.
type ValidateSuite struct {
	base.CLISuite

	tmpDir   string
	savedCwd string
}

// SuiteName ...
func (suite *ValidateSuite) SuiteName() string {
	return "cli.ValidateSuite"
}

// SetupTest ...
func (suite *ValidateSuite) SetupTest() {
	suite.tmpDir = suite.T().TempDir()

	var err error

	suite.savedCwd, err = os.Getwd()
	suite.Require().NoError(err)

	suite.Require().NoError(os.Chdir(suite.tmpDir))
}

// TearDownTest ...
func (suite *ValidateSuite) TearDownTest() {
	if suite.savedCwd != "" {
		suite.Require().NoError(os.Chdir(suite.savedCwd))
	}
}

// TestValidate generates config and validates it for all the modes.
func (suite *ValidateSuite) TestValidate() {
	suite.RunCLI([]string{"gen", "config", "foobar", "https://10.0.0.1"},
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
	)

	for _, configFile := range []string{"controlplane.yaml", "worker.yaml"} {
		for _, mode := range []string{"cloud", "metal"} {
			suite.Run(fmt.Sprintf("%s-%s", configFile, mode), func() {
				suite.RunCLI([]string{"validate", "-m", mode, "-c", configFile, "--strict"})
			})
		}
	}
}

func init() {
	allSuites = append(allSuites, new(ValidateSuite))
}
