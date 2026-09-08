// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// LLDPSuite verifies receipt from the provisioner-owned --with-lldp fixture.
// The runner uses only the Talos API, including when the QEMU host is remote.
type LLDPSuite struct {
	base.APISuite
}

// SuiteName implements base.Suite.
func (suite *LLDPSuite) SuiteName() string {
	return "api.LLDPSuite"
}

// TestListener checks all seven neighbor fields of the known advertisement.
// Update, withdrawal and expiry are covered deterministically by controller tests.
func (suite *LLDPSuite) TestListener() {
	if !suite.LLDPEnabled {
		suite.T().Skip("enable with -talos.lldp (requires a cluster created with --with-lldp)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	nodeCtx := client.WithNode(ctx, suite.RandomDiscoveredNodeInternalIP())
	links, err := safe.StateListAll[*network.LinkStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	for link := range links.All() {
		if !link.TypedSpec().Physical() || len(link.TypedSpec().HardwareAddr) == 0 {
			continue
		}

		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, link.Metadata().ID(), func(status *network.LLDPNeighborStatus, assertions *assert.Assertions) {
			assertions.Equal([]network.LLDPNeighborSpec{{
				ChassisID:           "02:00:00:00:00:01",
				SystemName:          "switch.example.com",
				SystemDescription:   "test switch",
				PortID:              "swp-test1",
				PortDescription:     "Ethernet test port",
				ManagementAddresses: []string{"192.0.2.1", "2001:db8::1"},
				VLANs:               []network.LLDPVLANSpec{{ID: 100, Name: "prod"}, {ID: 200, Name: "storage"}},
			}}, status.TypedSpec().Neighbors)
		}, rtestutils.WithNamespace(network.NamespaceName))

		return
	}

	suite.FailNow("no physical Ethernet link found for LLDP test")
}

func init() {
	allSuites = append(allSuites, new(LLDPSuite))
}
