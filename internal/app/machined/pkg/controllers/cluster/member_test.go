// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net/netip"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

type MemberSuite struct {
	ClusterSuite
}

func (suite *MemberSuite) TestReconcileDefault() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.MemberController{}))

	affiliate1 := cluster.NewAffiliate(cluster.NamespaceName, "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
	*affiliate1.TypedSpec() = cluster.AffiliateSpec{
		NodeID:          "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Hostname:        "foo.com",
		Nodename:        "bar",
		MachineType:     machine.TypeControlPlane,
		OperatingSystem: "Talos (v1.0.0)",
		Addresses:       []netip.Addr{netip.MustParseAddr("192.168.3.4")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
			Address:             netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
			AdditionalAddresses: []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")},
			Endpoints:           []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
		},
		ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
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
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.6")},
	}

	for _, r := range []resource.Resource{affiliate1, affiliate2, affiliate3} {
		suite.Require().NoError(suite.state.Create(suite.ctx, r))
	}

	// affiliates with non-empty Nodename should be translated to Members
	ctest.AssertResource(
		suite,
		affiliate1.TypedSpec().Nodename,
		func(r *cluster.Member, asrt *assert.Assertions) {
			spec := r.TypedSpec()

			asrt.Equal(affiliate1.TypedSpec().NodeID, spec.NodeID)
			asrt.Equal([]netip.Addr{netip.MustParseAddr("192.168.3.4")}, spec.Addresses)
			asrt.Equal("foo.com", spec.Hostname)
			asrt.Equal(machine.TypeControlPlane, spec.MachineType)
			asrt.Equal("Talos (v1.0.0)", spec.OperatingSystem)
			asrt.Equal(6443, spec.ControlPlane.APIServerPort)
		},
	)

	ctest.AssertResource(
		suite,
		affiliate2.TypedSpec().Nodename,
		func(r *cluster.Member, asrt *assert.Assertions) {
			spec := r.TypedSpec()

			asrt.Equal(affiliate2.TypedSpec().NodeID, spec.NodeID)
			asrt.Equal([]netip.Addr{netip.MustParseAddr("192.168.3.5")}, spec.Addresses)
			asrt.Equal("worker-1", spec.Hostname)
			asrt.Equal(machine.TypeWorker, spec.MachineType)
		},
	)

	// remove affiliate2, member information should eventually go away
	suite.Require().NoError(suite.state.Destroy(suite.ctx, affiliate2.Metadata()))

	ctest.AssertNoResource[*cluster.Member](suite, affiliate2.TypedSpec().Nodename)
}

func TestMemberSuite(t *testing.T) {
	suite.Run(t, new(MemberSuite))
}
