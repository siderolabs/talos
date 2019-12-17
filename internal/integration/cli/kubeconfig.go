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

// TestSuccess runs comand with success.
func (suite *KubeconfigSuite) TestSuccess() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint: errcheck

	suite.RunOsctl([]string{"kubeconfig", tempDir},
		base.StdoutEmpty())

	_, err = os.Stat(filepath.Join(tempDir, "kubeconfig"))
	suite.Require().NoError(err)
}

// TestMultiNodeFail verifies that command fails with multiple nodes.
func (suite *KubeconfigSuite) TestMultiNodeFail() {
	suite.RunOsctl([]string{"kubeconfig", "--nodes", "127.0.0.1", "--nodes", "127.0.0.1", "."},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`is not supported with multiple nodes`)))
}

func init() {
	allSuites = append(allSuites, new(KubeconfigSuite))
}
