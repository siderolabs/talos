// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"context"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

// NetworkConfigSuite ...
type NetworkConfigSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *NetworkConfigSuite) SuiteName() string {
	return "api.NetworkConfigSuite"
}

// SetupTest ...
func (suite *NetworkConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)
}

// TearDownTest ...
func (suite *NetworkConfigSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestStaticHostConfig tests that /etc/hosts updates are working.
func (suite *NetworkConfigSuite) TestStaticHostConfig() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	host1 := network.NewStaticHostConfigV1Alpha1("1.2.3.4")
	host1.Hostnames = []string{"example.com", "example2"}

	host2 := network.NewStaticHostConfigV1Alpha1("2001:db8::1")
	host2.Hostnames = []string{"v6"}

	suite.PatchMachineConfig(nodeCtx, host1, host2)

	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			hosts := suite.ReadFile(nodeCtx, "/etc/hosts")
			scanner := bufio.NewScanner(strings.NewReader(hosts))

			var found1, found2 bool

			for scanner.Scan() {
				line := scanner.Text()

				switch {
				case strings.HasPrefix(line, "1.2.3.4"):
					found1 = true

					asrt.Contains(line, "example.com", "expected to find hostname in IPv4 entry")
					asrt.Contains(line, "example2", "expected to find hostname in IPv4 entry")
				case strings.HasPrefix(line, "2001:db8::1"):
					found2 = true

					asrt.Contains(line, "v6", "expected to find hostname in IPv6 entry")
				}
			}

			asrt.True(found1, "expected to find IPv4 entry in /etc/hosts")
			asrt.True(found2, "expected to find IPv6 entry in /etc/hosts")
		},
		time.Second, time.Millisecond, "waiting for /etc/hosts to be updated",
	)

	suite.RemoveMachineConfigDocuments(nodeCtx, network.StaticHostKind)

	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			hosts := suite.ReadFile(nodeCtx, "/etc/hosts")

			asrt.NotContains(hosts, "1.2.3.4", "expected to not find IPv4 entry in /etc/hosts")
			asrt.NotContains(hosts, "2001:db8::1", "expected to not find IPv6 entry in /etc/hosts")
		},
		time.Second, time.Millisecond, "waiting for /etc/hosts to be updated",
	)
}

func init() {
	allSuites = append(allSuites, new(NetworkConfigSuite))
}
