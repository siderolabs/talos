// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HardwareAddrSuite struct {
	ctest.DefaultSuite
}

func (suite *HardwareAddrSuite) TestFirst() {
	mustParseMAC := func(addr string) nethelpers.HardwareAddr {
		mac, err := net.ParseMAC(addr)
		suite.Require().NoError(err)

		return nethelpers.HardwareAddr(mac)
	}

	eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
	eth0.TypedSpec().Type = nethelpers.LinkEther
	eth0.TypedSpec().HardwareAddr = mustParseMAC("56:a0:a0:87:1c:fa")

	eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
	eth1.TypedSpec().Type = nethelpers.LinkEther
	eth1.TypedSpec().HardwareAddr = mustParseMAC("6a:2b:bd:b2:fc:e0")

	bond0 := network.NewLinkStatus(network.NamespaceName, "bond0")
	bond0.TypedSpec().Type = nethelpers.LinkEther
	bond0.TypedSpec().Kind = "bond"
	bond0.TypedSpec().HardwareAddr = mustParseMAC("56:a0:a0:87:1c:fb")

	suite.Create(bond0)
	suite.Create(eth1)

	ctest.AssertResource(
		suite,
		network.FirstHardwareAddr,
		func(r *network.HardwareAddr, asrt *assert.Assertions) {
			asrt.Equal(eth1.Metadata().ID(), r.TypedSpec().Name)
			asrt.Equal("6a:2b:bd:b2:fc:e0", net.HardwareAddr(r.TypedSpec().HardwareAddr).String())
		},
	)

	suite.Create(eth0)

	ctest.AssertResource(
		suite,
		network.FirstHardwareAddr,
		func(r *network.HardwareAddr, asrt *assert.Assertions) {
			asrt.Equal(eth0.Metadata().ID(), r.TypedSpec().Name)
			asrt.Equal("56:a0:a0:87:1c:fa", net.HardwareAddr(r.TypedSpec().HardwareAddr).String())
		},
	)

	suite.Destroy(eth0)
	suite.Destroy(eth1)

	ctest.AssertNoResource[*network.HardwareAddr](suite, network.FirstHardwareAddr)
}

func TestHardwareAddrSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &HardwareAddrSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HardwareAddrController{}))
			},
		},
	})
}
