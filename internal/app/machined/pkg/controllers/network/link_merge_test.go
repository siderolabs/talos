// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type LinkMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *LinkMergeSuite) assertLinks(requiredIDs []string, check func(*network.LinkSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *LinkMergeSuite) assertNoLinks(id string) {
	ctest.AssertNoResource[*network.LinkSpec](suite, id)
}

func (suite *LinkMergeSuite) TestMerge() {
	loopback := network.NewLinkSpec(network.ConfigNamespaceName, "default/lo")
	*loopback.TypedSpec() = network.LinkSpecSpec{
		Name:        "lo",
		Up:          true,
		ConfigLayer: network.ConfigDefault,
	}

	dhcp := network.NewLinkSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1450,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewLinkSpec(network.ConfigNamespaceName, "configuration/eth0")
	*static.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1500,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{loopback, dhcp, static} {
		suite.Create(res)
	}

	suite.assertLinks(
		[]string{
			"lo",
			"eth0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "lo":
				asrt.Equal(*loopback.TypedSpec(), *r.TypedSpec())
			case "eth0":
				asrt.EqualValues(1500, r.TypedSpec().MTU) // static should override dhcp
			}
		},
	)

	suite.Destroy(static)

	suite.assertLinks(
		[]string{
			"lo",
			"eth0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "lo":
				asrt.Equal(*loopback.TypedSpec(), *r.TypedSpec())
			case "eth0":
				// reconcile happens eventually, so give it some time
				asrt.EqualValues(1450, r.TypedSpec().MTU)
			}
		},
	)

	suite.Destroy(loopback)

	suite.assertNoLinks("lo")
}

func (suite *LinkMergeSuite) TestMergeLogicalLink() {
	bondPlatform := network.NewLinkSpec(network.ConfigNamespaceName, "platform/bond0")
	*bondPlatform.TypedSpec() = network.LinkSpecSpec{
		Name:    "bond0",
		Logical: true,
		Up:      true,
		BondMaster: network.BondMasterSpec{
			Mode: nethelpers.BondMode8023AD,
		},
		ConfigLayer: network.ConfigPlatform,
	}

	bondMachineConfig := network.NewLinkSpec(network.ConfigNamespaceName, "config/bond0")
	*bondMachineConfig.TypedSpec() = network.LinkSpecSpec{
		Name:        "bond0",
		MTU:         1450,
		Up:          true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{bondPlatform, bondMachineConfig} {
		suite.Create(res)
	}

	suite.assertLinks(
		[]string{
			"bond0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.True(r.TypedSpec().Logical)
			asrt.EqualValues(1450, r.TypedSpec().MTU)
		},
	)
}

func (suite *LinkMergeSuite) TestMergeFlapping() {
	// simulate two conflicting link definitions which are getting removed/added constantly
	dhcp := network.NewLinkSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1450,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewLinkSpec(network.ConfigNamespaceName, "configuration/eth0")
	*static.TypedSpec() = network.LinkSpecSpec{
		Name:        "eth0",
		Up:          true,
		MTU:         1500,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	testMergeFlapping(&suite.DefaultSuite, []*network.LinkSpec{dhcp, static}, "eth0", static)
}

func (suite *LinkMergeSuite) TestMergeWireguard() {
	static := network.NewLinkSpec(network.ConfigNamespaceName, "configuration/kubespan")
	*static.TypedSpec() = network.LinkSpecSpec{
		Name: "kubespan",
		Wireguard: network.WireguardSpec{
			ListenPort: 1234,
			Peers: []network.WireguardPeer{
				{
					PublicKey: "bGsc2rOpl6JHd/Pm4fYrIkEABL0ZxW7IlaSyh77IMhw=",
					Endpoint:  "127.0.0.1:9999",
				},
			},
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	kubespanOperator := network.NewLinkSpec(network.ConfigNamespaceName, "kubespan/kubespan")
	*kubespanOperator.TypedSpec() = network.LinkSpecSpec{
		Name: "kubespan",
		Wireguard: network.WireguardSpec{
			PrivateKey: "IG9MqCII7z54Ysof1fQ9a7WcMNG+qNJRMyRCQz3JTUY=",
			ListenPort: 3456,
			Peers: []network.WireguardPeer{
				{
					PublicKey: "RXdQkMTD1Jcxd/Wizr9k8syw8ANs57l5jTormDVHAVs=",
					Endpoint:  "127.0.0.1:1234",
				},
			},
		},
		ConfigLayer: network.ConfigOperator,
	}

	for _, res := range []resource.Resource{static, kubespanOperator} {
		suite.Create(res)
	}

	suite.assertLinks(
		[]string{
			"kubespan",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(
				"IG9MqCII7z54Ysof1fQ9a7WcMNG+qNJRMyRCQz3JTUY=",
				r.TypedSpec().Wireguard.PrivateKey,
			)
			asrt.Equal(1234, r.TypedSpec().Wireguard.ListenPort)
			asrt.Len(r.TypedSpec().Wireguard.Peers, 2)

			if len(r.TypedSpec().Wireguard.Peers) != 2 {
				return
			}

			asrt.Equal(
				network.WireguardPeer{
					PublicKey: "RXdQkMTD1Jcxd/Wizr9k8syw8ANs57l5jTormDVHAVs=",
					Endpoint:  "127.0.0.1:1234",
				},
				r.TypedSpec().Wireguard.Peers[0],
			)

			asrt.Equal(
				network.WireguardPeer{
					PublicKey: "bGsc2rOpl6JHd/Pm4fYrIkEABL0ZxW7IlaSyh77IMhw=",
					Endpoint:  "127.0.0.1:9999",
				},
				r.TypedSpec().Wireguard.Peers[1],
			)
		},
	)

	suite.Destroy(kubespanOperator)
	suite.Destroy(static)

	suite.assertNoLinks("kubespan")
}

func TestLinkMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LinkMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(netctrl.NewLinkMergeController()))
			},
		},
	})
}
