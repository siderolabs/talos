// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// KubeconfigSuite verifies dmesg command
type KubeconfigSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *KubeconfigSuite) SuiteName() string {
	return "cli.KubeconfigSuite"
}

// TestDirectory generates kubeconfig in specified directory.
func (suite *KubeconfigSuite) TestDirectory() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint: errcheck

	suite.RunCLI([]string{"kubeconfig", tempDir},
		base.StdoutEmpty())

	suite.Require().FileExists(filepath.Join(tempDir, "kubeconfig"))

	// TODO: launch kubectl with config to verify it
}

// TestCwd generates kubeconfig in cwd.
func (suite *KubeconfigSuite) TestCwd() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint: errcheck

	savedCwd, err := os.Getwd()
	suite.Require().NoError(err)

	defer os.Chdir(savedCwd) //nolint: errcheck

	suite.Require().NoError(os.Chdir(tempDir))

	suite.RunCLI([]string{"kubeconfig"},
		base.StdoutEmpty())

	suite.Require().FileExists(filepath.Join(tempDir, "kubeconfig"))
}

// TestMultiNodeFail verifies that command fails with multiple nodes.
func (suite *KubeconfigSuite) TestMultiNodeFail() {
	suite.RunCLI([]string{"kubeconfig", "--nodes", "127.0.0.1", "--nodes", "127.0.0.1", "."},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`is not supported with multiple nodes`)))
}

func init() {
	allSuites = append(allSuites, new(KubeconfigSuite))
}
