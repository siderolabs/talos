// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type NodeIPSuite struct {
	ctest.DefaultSuite
}

func (suite *NodeIPSuite) TestReconcileIPv4() {
	cfg := k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().ValidSubnets = []string{"10.0.0.0/24", "::/0"}
	cfg.TypedSpec().ExcludeSubnets = []string{"10.0.0.2"}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	addresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s),
	)

	addresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.2/32"), // excluded explicitly
		netip.MustParsePrefix("10.0.0.5/24"),
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), addresses))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(nodeIP *k8s.NodeIP, asrt *assert.Assertions) {
		asrt.Equal("[10.0.0.5]", fmt.Sprintf("%s", nodeIP.TypedSpec().Addresses))
	})
}

func (suite *NodeIPSuite) TestReconcileDefaultSubnets() {
	cfg := k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().ValidSubnets = []string{"0.0.0.0/0", "::/0"}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	addresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s),
	)
	addresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.5/24"),
		netip.MustParsePrefix("192.168.1.1/24"),
		netip.MustParsePrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64"),
		netip.MustParsePrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7335/64"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), addresses))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(nodeIP *k8s.NodeIP, asrt *assert.Assertions) {
		asrt.Equal("[10.0.0.5 2001:db8:85a3::8a2e:370:7334]", fmt.Sprintf("%s", nodeIP.TypedSpec().Addresses))
	})
}

func (suite *NodeIPSuite) TestReconcileNoMatch() {
	cfg := k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().ValidSubnets = []string{"0.0.0.0/0"}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	addresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s),
	)
	addresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.2/32"),
		netip.MustParsePrefix("10.0.0.5/24"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), addresses))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(nodeIP *k8s.NodeIP, asrt *assert.Assertions) {
		asrt.Equal("[10.0.0.2]", fmt.Sprintf("%s", nodeIP.TypedSpec().Addresses))
	})

	cfg.TypedSpec().ValidSubnets = nil
	cfg.TypedSpec().ExcludeSubnets = []string{"10.0.0.2"}
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	// the node IP doesn't change, as there's no match for the filter
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(nodeIP *k8s.NodeIP, asrt *assert.Assertions) {
		asrt.Equal("[10.0.0.2]", fmt.Sprintf("%s", nodeIP.TypedSpec().Addresses))
	})
}

func (suite *NodeIPSuite) TestReconcileIPv6Denies() {
	cfg := k8s.NewNodeIPConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().ValidSubnets = []string{"::/0", "!fd01:cafe::f14c:9fa1:8496:557f/128"}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	addresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s),
	)

	addresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("fd01:cafe::f14c:9fa1:8496:557f/128"),
		netip.MustParsePrefix("fd01:cafe::5054:ff:fe1f:c7bd/64"),
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), addresses))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{k8s.KubeletID}, func(nodeIP *k8s.NodeIP, asrt *assert.Assertions) {
		asrt.Equal("[fd01:cafe::5054:ff:fe1f:c7bd]", fmt.Sprintf("%s", nodeIP.TypedSpec().Addresses))
	})
}

func TestNodeIPSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &NodeIPSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&k8sctrl.NodeIPController{}))
			},
		},
	})
}
