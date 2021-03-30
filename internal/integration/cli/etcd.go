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
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// EtcdSuite verifies etcd command.
type EtcdSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *EtcdSuite) SuiteName() string {
	return "cli.EtcdSuite"
}

// TestMembers etcd members should have some output.
func (suite *EtcdSuite) TestMembers() {
	suite.RunCLI([]string{"etcd", "members", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane)}) // default checks for stdout not empty
}

// TestForfeitLeadership etcd forfeit-leadership check.
func (suite *EtcdSuite) TestForfeitLeadership() {
	nodes := suite.DiscoverNodes().NodesByType(machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	suite.RunCLI([]string{"etcd", "forfeit-leadership", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane)},
		base.StdoutEmpty(),
	)
}

// TestSnapshot tests etcd snapshot (backup).
func (suite *EtcdSuite) TestSnapshot() {
	tempDir, err := ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	defer func() {
		suite.Assert().NoError(os.RemoveAll(tempDir))
	}()

	dbPath := filepath.Join(tempDir, "snapshot.db")

	suite.RunCLI([]string{"etcd", "snapshot", dbPath, "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`etcd snapshot saved to .+\d+ bytes.+`)),
	)
}

func init() {
	allSuites = append(allSuites, new(EtcdSuite))
}
