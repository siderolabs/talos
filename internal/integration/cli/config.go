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
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

// TalosconfigSuite verifies dmesg command.
type TalosconfigSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *TalosconfigSuite) SuiteName() string {
	return "cli.TalosconfigSuite"
}

// TestList checks how talosctl config merge.
func (suite *TalosconfigSuite) TestList() {
	suite.RunCLI([]string{"config", "contexts"},
		base.StdoutShouldMatch(regexp.MustCompile(`CURRENT`)))
}

// TestMerge checks how talosctl config merge.
func (suite *TalosconfigSuite) TestMerge() {
	tempDir, err := ioutil.TempDir("", "talos")
	defer os.RemoveAll(tempDir) //nolint:errcheck

	suite.Require().NoError(err)

	suite.RunCLI([]string{"gen", "config", "-o", tempDir, "foo", "https://192.168.0.1:6443"})

	talosconfigPath := filepath.Join(tempDir, "talosconfig")

	suite.Assert().FileExists(talosconfigPath)

	path := filepath.Join(tempDir, "merged")

	suite.RunCLI([]string{"config", "merge", "--talosconfig", path, talosconfigPath},
		base.StdoutEmpty())

	suite.Require().FileExists(path)

	c, err := clientconfig.Open(path)
	suite.Require().NoError(err)

	suite.Require().NotNil(c.Contexts["foo"])

	suite.RunCLI([]string{"config", "merge", "--talosconfig", path, talosconfigPath},
		base.StdoutShouldMatch(regexp.MustCompile(`renamed`)))

	c, err = clientconfig.Open(path)
	suite.Require().NoError(err)

	suite.Require().NotNil(c.Contexts["foo-1"])
}

func init() {
	allSuites = append(allSuites, new(TalosconfigSuite))
}
