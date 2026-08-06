// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"bytes"
	"strings"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// PcapSuite verifies etcd command.
type PcapSuite struct {
	base.CLISuite
}

const trafficGenDelay = 250 * time.Millisecond

// SuiteName ...
func (suite *PcapSuite) SuiteName() string {
	return "cli.PcapSuite"
}

// TestLoopback verifies that loopback traffic can be captured reliably.
func (suite *PcapSuite) TestLoopback() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	cmd := suite.MakeCMDFn([]string{"pcap", "--interface", "lo", "--nodes", node, "--duration", "3s"})()

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	suite.Require().NoError(cmd.Start())

	// Give pcap a moment to initialize before generating loopback traffic.
	time.Sleep(trafficGenDelay)

	for range 3 {
		suite.RunCLI([]string{"read", "--nodes", node, "/proc/net/dev"})
		time.Sleep(trafficGenDelay)
	}

	suite.Require().NoError(cmd.Wait(), "pcap failed, stdout: %q, stderr: %q", stdout.String(), stderr.String())
	suite.Assert().NotEmpty(strings.TrimSpace(stdout.String()), "stdout should be not empty")
	suite.Assert().Empty(stderr.String(), "stderr should be empty")
}

func init() {
	allSuites = append(allSuites, new(PcapSuite))
}
