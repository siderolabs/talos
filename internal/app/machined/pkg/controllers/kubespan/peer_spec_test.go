// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusteradapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	kubespanctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type PeerSpecSuite struct {
	ctest.DefaultSuite

	statePath string
}

func (suite *PeerSpecSuite) TestReconcile() {
	suite.statePath = suite.T().TempDir()

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)
	suite.Create(stateMount)

	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	suite.Create(cfg)

	nodeIdentity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	suite.Require().NoError(clusteradapter.IdentitySpec(nodeIdentity.TypedSpec()).Generate())
	suite.Create(nodeIdentity)

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
			AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24"), netip.MustParsePrefix("10.244.3.0/32")},
			Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
		},
	}

	affiliate2 := cluster.NewAffiliate(cluster.NamespaceName, "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F")
	*affiliate2.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F",
		Hostname:    "worker-1",
		Nodename:    "worker-1",
		MachineType: machine.TypeWorker,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.5")},
	}

	affiliate3 := cluster.NewAffiliate(cluster.NamespaceName, "xCnFFfxylOf9i5ynhAkt6ZbfcqaLDGKfIa3gwpuaxe7F")
	*affiliate3.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "xCnFFfxylOf9i5ynhAkt6ZbfcqaLDGKfIa3gwpuaxe7F",
		MachineType: machine.TypeWorker,
		Nodename:    "worker-2",
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.6")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "mB6WlFOR66Jx5rtPMIpxJ3s4XHyer9NCzqWPP7idGRo",
			Address:             netip.MustParseAddr("fdc8:8aee:4e2d:1202:f073:9cff:fe6c:4d67"),
			AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.4.1/24")},
			Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("192.168.3.6:51820")},
		},
	}

	// local node affiliate, should be skipped as a peer
	affiliate4 := cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID)
	*affiliate4.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      nodeIdentity.TypedSpec().NodeID,
		MachineType: machine.TypeWorker,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.7")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "27E8I+ekrqT21cq2iW6+fDe+H7WBw6q9J7vqLCeswiM=",
			Address:             netip.MustParseAddr("fdc8:8aee:4e2d:1202:f073:9cff:fe6c:4d67"),
			AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.5.1/24")},
			Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("192.168.3.7:51820")},
		},
	}

	for _, r := range []resource.Resource{affiliate1, affiliate2, affiliate3, affiliate4} {
		suite.Create(r)
	}

	// affiliate2 shouldn't be rendered as a peer, as it doesn't have kubespan data
	ctest.AssertResources(suite,
		[]resource.ID{
			affiliate1.TypedSpec().KubeSpan.PublicKey,
			affiliate3.TypedSpec().KubeSpan.PublicKey,
		},
		func(*kubespan.PeerSpec, *assert.Assertions) {},
	)
	ctest.AssertNoResource[*kubespan.PeerSpec](suite, affiliate2.TypedSpec().KubeSpan.PublicKey)

	ctest.AssertResource(suite,
		affiliate1.TypedSpec().KubeSpan.PublicKey,
		func(res *kubespan.PeerSpec, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0", spec.Address.String())
			asrt.Equal("[10.244.3.0/24 192.168.3.4/32 fd50:8d60:4238:6302:f857:23ff:fe21:d1e0/128]", fmt.Sprintf("%v", spec.AllowedIPs))
			asrt.Equal([]netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")}, spec.Endpoints)
			asrt.Equal("bar", spec.Label)
		},
	)

	ctest.AssertResource(suite,
		affiliate3.TypedSpec().KubeSpan.PublicKey,
		func(res *kubespan.PeerSpec, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal("fdc8:8aee:4e2d:1202:f073:9cff:fe6c:4d67", spec.Address.String())
			asrt.Equal("[10.244.4.0/24 192.168.3.6/32 fdc8:8aee:4e2d:1202:f073:9cff:fe6c:4d67/128]", fmt.Sprintf("%v", spec.AllowedIPs))
			asrt.Equal([]netip.AddrPort{netip.MustParseAddrPort("192.168.3.6:51820")}, spec.Endpoints)
			asrt.Equal("worker-2", spec.Label)
		},
	)

	// disabling kubespan should remove all peers
	cfg.TypedSpec().Enabled = false
	suite.Update(cfg)

	ctest.AssertNoResource[*kubespan.PeerSpec](suite, affiliate1.TypedSpec().KubeSpan.PublicKey)
	ctest.AssertNoResource[*kubespan.PeerSpec](suite, affiliate2.TypedSpec().KubeSpan.PublicKey)
}

func (suite *PeerSpecSuite) TestIPOverlap() {
	suite.statePath = suite.T().TempDir()

	stateMount := runtimeres.NewMountStatus(v1alpha1.NamespaceName, constants.StatePartitionLabel)
	suite.Create(stateMount)

	cfg := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	cfg.TypedSpec().Enabled = true
	suite.Create(cfg)

	nodeIdentity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	suite.Require().NoError(clusteradapter.IdentitySpec(nodeIdentity.TypedSpec()).Generate())
	suite.Create(nodeIdentity)

	affiliate1 := cluster.NewAffiliate(cluster.NamespaceName, "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
	*affiliate1.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
			Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
			AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24"), netip.MustParsePrefix("10.244.3.0/32")},
			Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
		},
	}

	affiliate2 := cluster.NewAffiliate(cluster.NamespaceName, "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F")
	*affiliate2.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F",
		Hostname:    "worker-1",
		Nodename:    "worker-1",
		MachineType: machine.TypeWorker,
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "Zr5ewpUm2Ywo1c+/59WFKIBjZ3c/nVbIWsT5elbjwCU=",
			Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e1"),
			AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.2.0/23"), netip.MustParsePrefix("192.168.3.0/24")},
			Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
		},
	}

	for _, r := range []resource.Resource{affiliate1, affiliate2} {
		suite.Create(r)
	}

	// affiliate2 should be rendered as a peer, but with reduced address as its AdditionalAddresses overlap with affiliate1 addresses
	ctest.AssertResource(suite,
		affiliate1.TypedSpec().KubeSpan.PublicKey,
		func(res *kubespan.PeerSpec, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(`["10.244.3.0/24" "fd50:8d60:4238:6302:f857:23ff:fe21:d1e0/128"]`, fmt.Sprintf("%q", spec.AllowedIPs))
		},
	)

	ctest.AssertResource(suite,
		affiliate2.TypedSpec().KubeSpan.PublicKey,
		func(res *kubespan.PeerSpec, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal(`["10.244.2.0/24" "192.168.3.0/24" "fd50:8d60:4238:6302:f857:23ff:fe21:d1e1/128"]`, fmt.Sprintf("%q", spec.AllowedIPs))
		},
	)
}

func TestPeerSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &PeerSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&kubespanctrl.PeerSpecController{}))
			},
		},
	})
}
