// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"net/netip"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	nodes := suite.DiscoverNodeInternalIPs(ctx)
	require.NotEmpty(suite.T(), nodes)

	portIDs := make(map[string]string, len(nodes))

	for _, node := range nodes {
		nodeCtx := client.WithNode(ctx, node)
		nodeIP, err := netip.ParseAddr(node)
		require.NoError(suite.T(), err)

		addresses, err := safe.StateListAll[*network.AddressStatus](nodeCtx, suite.Client.COSI)
		require.NoError(suite.T(), err)

		// The provisioned node IP belongs to net0, the management bridge NIC.
		// Resolve it through AddressStatus rather than relying on link order or names:
		// extra physical NICs can connect to fabric links without this advertiser.
		var managementLink string

		for address := range addresses.All() {
			if address.TypedSpec().Address.Addr().Unmap() == nodeIP.Unmap() {
				managementLink = address.TypedSpec().LinkName

				break
			}
		}

		require.NotEmpty(suite.T(), managementLink, "node %s has no interface with its management IP", node)

		link, err := safe.ReaderGetByID[*network.LinkStatus](nodeCtx, suite.Client.COSI, managementLink)
		require.NoError(suite.T(), err)
		require.True(suite.T(), link.TypedSpec().Physical(), "node %s management link %s is not physical", node, managementLink)
		require.NotEmpty(suite.T(), link.TypedSpec().HardwareAddr)

		var portID string

		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, managementLink, func(status *network.LLDPNeighborStatus, assertions *assert.Assertions) {
			if !assertions.Len(status.TypedSpec().Neighbors, 1) {
				return
			}

			neighbor := status.TypedSpec().Neighbors[0]
			assertions.Equal("02:00:00:00:00:01", neighbor.ChassisID)
			assertions.Equal("switch.example.com", neighbor.SystemName)
			assertions.Equal("test switch", neighbor.SystemDescription)
			assertions.NotEmpty(neighbor.PortID)
			assertions.Equal("Ethernet test port", neighbor.PortDescription)
			assertions.Equal([]string{"192.0.2.1", "2001:db8::1"}, neighbor.ManagementAddresses)
			assertions.Equal([]network.LLDPVLANSpec{{ID: 100, Name: "prod"}, {ID: 200, Name: "storage"}}, neighbor.VLANs)
			portID = neighbor.PortID
		}, rtestutils.WithNamespace(network.NamespaceName))

		require.NotEmpty(suite.T(), portID)
		assert.NotContains(suite.T(), portIDs, portID, "node %s and node %s advertised the same port ID", node, portIDs[portID])
		portIDs[portID] = node
	}
}

func init() {
	allSuites = append(allSuites, new(LLDPSuite))
}
