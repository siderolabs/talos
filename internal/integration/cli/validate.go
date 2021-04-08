// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/talos-systems/talos/internal/integration/base"
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
	var err error
	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.savedCwd, err = os.Getwd()
	suite.Require().NoError(err)

	suite.Require().NoError(os.Chdir(suite.tmpDir))
}

// TearDownTest ...
func (suite *ValidateSuite) TearDownTest() {
	if suite.savedCwd != "" {
		suite.Require().NoError(os.Chdir(suite.savedCwd))
	}

	if suite.tmpDir != "" {
		suite.Require().NoError(os.RemoveAll(suite.tmpDir))
	}
}

// TestValidate generates config and validates it for all the modes.
func (suite *ValidateSuite) TestValidate() {
	suite.RunCLI([]string{"gen", "config", "foobar", "https://10.0.0.1"})

	for _, configFile := range []string{"init.yaml", "controlplane.yaml", "join.yaml"} {
		configFile := configFile

		for _, mode := range []string{"cloud", "container"} {
			mode := mode

			suite.Run(fmt.Sprintf("%s-%s", configFile, mode), func() {
				suite.RunCLI([]string{"validate", "-m", mode, "-c", configFile, "--strict"})
			})
		}
	}
}

func init() {
	allSuites = append(allSuites, new(ValidateSuite))
}
