// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

type AffiliateMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *AffiliateMergeSuite) TestReconcileDefault() {
	suite.Require().NoError(suite.Runtime().RegisterController(&clusterctrl.AffiliateMergeController{}))

	affiliate1 := cluster.NewAffiliate(cluster.RawNamespaceName, "k8s/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
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
		ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
	}

	affiliate2 := cluster.NewAffiliate(cluster.RawNamespaceName, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
	*affiliate2.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Hostname:    "foo.com",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.4"), netip.MustParseAddr("10.5.0.2")},
	}

	affiliate3 := cluster.NewAffiliate(cluster.RawNamespaceName, "service/9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F")
	*affiliate3.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F",
		Hostname:    "worker-1",
		Nodename:    "worker-1",
		MachineType: machine.TypeWorker,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.5")},
	}

	suite.Create(affiliate1)
	suite.Create(affiliate2)
	suite.Create(affiliate3)

	// there should be two merged affiliates: one from affiliate1+affiliate2, and another from affiliate3
	ctest.AssertResource(suite, affiliate1.TypedSpec().NodeID, func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(affiliate1.TypedSpec().NodeID, spec.NodeID)
		asrt.Equal([]netip.Addr{netip.MustParseAddr("192.168.3.4"), netip.MustParseAddr("10.5.0.2")}, spec.Addresses)
		asrt.Equal("foo.com", spec.Hostname)
		asrt.Equal("bar", spec.Nodename)
		asrt.Equal(machine.TypeControlPlane, spec.MachineType)
		asrt.Equal(netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"), spec.KubeSpan.Address)
		asrt.Equal("PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=", spec.KubeSpan.PublicKey)
		asrt.Equal([]netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")}, spec.KubeSpan.AdditionalAddresses)
		asrt.Equal([]netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")}, spec.KubeSpan.Endpoints)
		asrt.Equal(&cluster.ControlPlane{APIServerPort: 6443}, spec.ControlPlane)
	})

	ctest.AssertResource(suite, affiliate3.TypedSpec().NodeID, func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(affiliate3.TypedSpec().NodeID, spec.NodeID)
		asrt.Equal([]netip.Addr{netip.MustParseAddr("192.168.3.5")}, spec.Addresses)
		asrt.Equal("worker-1", spec.Hostname)
		asrt.Equal("worker-1", spec.Nodename)
		asrt.Equal(machine.TypeWorker, spec.MachineType)
		asrt.Zero(spec.KubeSpan.PublicKey)
		asrt.Nil(spec.ControlPlane)
	})

	// remove affiliate1, KubeSpan information should eventually go away
	suite.Destroy(affiliate1)

	ctest.AssertResource(suite, affiliate1.TypedSpec().NodeID, func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(affiliate1.TypedSpec().NodeID, spec.NodeID)
		asrt.Zero(spec.KubeSpan.Address)
		asrt.Zero(spec.KubeSpan.PublicKey)
		asrt.Zero(spec.KubeSpan.AdditionalAddresses)
		asrt.Zero(spec.KubeSpan.Endpoints)
		asrt.Nil(spec.ControlPlane)
	})

	// remove affiliate3, merged affiliate should be removed
	suite.Destroy(affiliate3)

	ctest.AssertNoResource[*cluster.Affiliate](suite, affiliate3.TypedSpec().NodeID)
}

func TestAffiliateMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &AffiliateMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
