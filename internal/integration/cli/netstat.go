// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"context"
	"regexp"
	"strings"

	"github.com/siderolabs/talos/internal/integration/base"
)

// NetstatSuite verifies etcd command and coredns container.
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

// TestContainers verifies that containers are listed.
func (suite *NetstatSuite) TestContainers() {
	nodes := suite.DiscoverNodeInternalIPs(context.TODO())

	suite.RunCLI([]string{"netstat", "--listening", "--programs", "--udp", "--ipv6", "--pods", "--nodes", strings.Join(nodes, ",")},
		base.StdoutShouldMatch(regexp.MustCompile(`:::53\s+:::\*.+/coredns\s+kube-system/coredns-`)))
}

func init() {
	allSuites = append(allSuites, new(NetstatSuite))
}
