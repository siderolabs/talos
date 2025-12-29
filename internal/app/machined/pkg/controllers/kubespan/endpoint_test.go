// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	kubespanctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

type EndpointSuite struct {
	ctest.DefaultSuite
}

func (suite *EndpointSuite) TestReconcile() {
	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().HarvestExtraEndpoints = true
	suite.Create(cfg)

	// create some affiliates and peer statuses
	affiliate1 := cluster.NewAffiliate(cluster.NamespaceName, "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
	*affiliate1.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Hostname:    "foo.com",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.4")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
			Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
			AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")},
			Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
		},
	}

	affiliate2 := cluster.NewAffiliate(cluster.NamespaceName, "roLng5hmP0Gv9S5Pbfzaa93JSZjsdpXNAn7vzuCfsc8")
	*affiliate2.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "roLng5hmP0Gv9S5Pbfzaa93JSZjsdpXNAn7vzuCfsc8",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.5")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey: "1CXkdhWBm58c36kTpchR8iGlXHG1ruHa5W8gsFqD8Qs=",
			Address:   netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e1"),
		},
	}

	suite.Create(affiliate1)
	suite.Create(affiliate2)

	peerStatus1 := kubespan.NewPeerStatus(kubespan.NamespaceName, affiliate1.TypedSpec().KubeSpan.PublicKey)
	*peerStatus1.TypedSpec() = kubespan.PeerStatusSpec{
		Endpoint: netip.MustParseAddrPort("10.3.4.8:278"),
		State:    kubespan.PeerStateUp,
	}

	peerStatus2 := kubespan.NewPeerStatus(kubespan.NamespaceName, affiliate2.TypedSpec().KubeSpan.PublicKey)
	*peerStatus2.TypedSpec() = kubespan.PeerStatusSpec{
		Endpoint: netip.MustParseAddrPort("10.3.4.9:279"),
		State:    kubespan.PeerStateUnknown,
	}

	peerStatus3 := kubespan.NewPeerStatus(kubespan.NamespaceName, "LoXPyyYh3kZwyKyWfCcf9VvgVv588cKhSKXavuUZqDg=")
	*peerStatus3.TypedSpec() = kubespan.PeerStatusSpec{
		Endpoint: netip.MustParseAddrPort("10.3.4.10:270"),
		State:    kubespan.PeerStateUp,
	}

	suite.Create(peerStatus1)
	suite.Create(peerStatus2)
	suite.Create(peerStatus3)

	// peer1 is up and has matching affiliate
	ctest.AssertResource(suite, peerStatus1.Metadata().ID(),
		func(res *kubespan.Endpoint, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(peerStatus1.TypedSpec().Endpoint, spec.Endpoint)
			asrt.Equal(affiliate1.TypedSpec().NodeID, spec.AffiliateID)
		},
	)

	// peer2 is not up, it shouldn't be published as an endpoint
	ctest.AssertNoResource[*kubespan.Endpoint](suite, peerStatus2.Metadata().ID())

	// peer3 is up, but has not matching affiliate
	ctest.AssertNoResource[*kubespan.Endpoint](suite, peerStatus3.Metadata().ID())
}

func TestEndpointSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &EndpointSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&kubespanctrl.EndpointController{}))
			},
		},
	})
}
