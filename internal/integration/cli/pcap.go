// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// PcapSuite verifies etcd command.
type PcapSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *PcapSuite) SuiteName() string {
	return "cli.PcapSuite"
}

// TestNodeTraffic verifies that packet capture can observe traffic on a stable interface.
func (suite *PcapSuite) TestNodeTraffic() {
	suite.RunCLI([]string{"pcap", "--interface", "eth0", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane), "--duration", "2s"}) // default checks for stdout not empty
}

func init() {
	allSuites = append(allSuites, new(PcapSuite))
}
