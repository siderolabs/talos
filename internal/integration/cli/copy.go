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

// CopySuite verifies dmesg command.
type CopySuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *CopySuite) SuiteName() string {
	return "cli.CopySuite"
}

// TestSuccess runs comand with success.
func (suite *CopySuite) TestSuccess() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer os.RemoveAll(tempDir) //nolint:errcheck

	suite.RunCLI([]string{"copy", "--nodes", suite.RandomDiscoveredNode(), "/etc/os-release", tempDir},
		base.StdoutEmpty())

	_, err = os.Stat(filepath.Join(tempDir, "os-release"))
	suite.Require().NoError(err)
}

// TestMultiNodeFail verifies that command fails with multiple nodes.
func (suite *CopySuite) TestMultiNodeFail() {
	suite.RunCLI([]string{"copy", "--nodes", "127.0.0.1", "--nodes", "127.0.0.1", "/etc/os-release", "."},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`is not supported with multiple nodes`)))
}

func init() {
	allSuites = append(allSuites, new(CopySuite))
}
