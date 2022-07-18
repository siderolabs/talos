// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// PcapSuite verifies etcd command.
type PcapSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *PcapSuite) SuiteName() string {
	return "cli.PcapSuite"
}

// TestLoopback verifies that there are some packets on loopback interface.
func (suite *PcapSuite) TestLoopback() {
	suite.RunCLI([]string{"pcap", "--interface", "lo", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane), "--duration", "1s"}) // default checks for stdout not empty
}

func init() {
	allSuites = append(allSuites, new(PcapSuite))
}
