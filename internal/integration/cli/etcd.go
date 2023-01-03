// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
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
	suite.RunCLI([]string{"etcd", "members", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)}) // default checks for stdout not empty
}

// TestStatus etcd status should have some output.
func (suite *EtcdSuite) TestStatus() {
	cpNodes := suite.DiscoverNodeInternalIPsByType(context.TODO(), machine.TypeControlPlane)

	suite.RunCLI([]string{"etcd", "status", "--nodes", strings.Join(cpNodes, ",")}) // default checks for stdout not empty
}

// TestAlarm etcd alarm should have no output.
func (suite *EtcdSuite) TestAlarm() {
	cpNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.RunCLI([]string{"etcd", "alarm", "list", "--nodes", cpNode}, base.StdoutEmpty())
	suite.RunCLI([]string{"etcd", "alarm", "disarm", "--nodes", cpNode}, base.StdoutEmpty())
}

// TestForfeitLeadership etcd forfeit-leadership check.
func (suite *EtcdSuite) TestForfeitLeadership() {
	nodes := suite.DiscoverNodes(context.TODO()).NodesByType(machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	suite.RunCLI([]string{"etcd", "forfeit-leadership", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)},
		base.StdoutEmpty(),
	)
}

// TestSnapshot tests etcd snapshot (backup).
func (suite *EtcdSuite) TestSnapshot() {
	tempDir := suite.T().TempDir()

	dbPath := filepath.Join(tempDir, "snapshot.db")

	suite.RunCLI([]string{"etcd", "snapshot", dbPath, "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`etcd snapshot saved to .+\d+ bytes.+`)),
	)
}

func init() {
	allSuites = append(allSuites, new(EtcdSuite))
}
