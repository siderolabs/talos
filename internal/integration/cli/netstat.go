// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"

	"github.com/siderolabs/talos/internal/integration/base"
)

// NetstatSuite verifies etcd command.
type NetstatSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *NetstatSuite) SuiteName() string {
	return "cli.NetstatSuite"
}

// TestListening verifies that there are listening connections.
func (suite *NetstatSuite) TestListening() {
	suite.RunCLI([]string{"netstat", "--listening", "--programs", "--tcp", "--extend", "--nodes", suite.RandomDiscoveredNodeInternalIP()},
		base.StdoutShouldMatch(regexp.MustCompile(`:::50000.+LISTEN.+/apid`)))
}

func init() {
	allSuites = append(allSuites, new(NetstatSuite))
}
