// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"path/filepath"
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// TalosconfigSuite checks `talosctl config`.
type TalosconfigSuite struct {
	base.CLISuite
}

// SuiteName implements base.NamedSuite.
func (suite *TalosconfigSuite) SuiteName() string {
	return "cli.TalosconfigSuite"
}

// TestList checks `talosctl config contexts`.
func (suite *TalosconfigSuite) TestList() {
	suite.RunCLI([]string{"config", "contexts"},
		base.StdoutShouldMatch(regexp.MustCompile(`CURRENT`)))
}

// TestMerge checks `talosctl config merge`.
func (suite *TalosconfigSuite) TestMerge() {
	tempDir := suite.T().TempDir()

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

// TestNew checks `talosctl config new`.
func (suite *TalosconfigSuite) TestNew() {
	tempDir := suite.T().TempDir()

	node := suite.RandomDiscoveredNode(machine.TypeControlPlane)

	talosconfigPath := filepath.Join(tempDir, "talosconfig")
	suite.RunCLI([]string{"--nodes", node, "config", "new", "--roles", "os:reader", talosconfigPath},
		base.StdoutEmpty())

	suite.RunCLI([]string{"--nodes", node, "--talosconfig", talosconfigPath, "ls", "/etc/hosts"},
		base.StdoutShouldMatch(regexp.MustCompile(`hosts`)))

	suite.RunCLI([]string{"--nodes", node, "--talosconfig", talosconfigPath, "read", "/etc/hosts"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)))
}

func init() {
	allSuites = append(allSuites, new(TalosconfigSuite))
}
