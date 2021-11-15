// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	clusterctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
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
		Addresses:       []netaddr.IP{netaddr.MustParseIP("192.168.3.4")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
			Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
			AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
			Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")},
		},
	}

	affiliate2 := cluster.NewAffiliate(cluster.NamespaceName, "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F")
	*affiliate2.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F",
		Hostname:    "worker-1",
		Nodename:    "worker-1",
		MachineType: machine.TypeWorker,
		Addresses:   []netaddr.IP{netaddr.MustParseIP("192.168.3.5")},
	}

	affiliate3 := cluster.NewAffiliate(cluster.NamespaceName, "xCnFFfxylOf9i5ynhAkt6ZbfcqaLDGKfIa3gwpuaxe7F")
	*affiliate3.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "xCnFFfxylOf9i5ynhAkt6ZbfcqaLDGKfIa3gwpuaxe7F",
		MachineType: machine.TypeWorker,
		Addresses:   []netaddr.IP{netaddr.MustParseIP("192.168.3.6")},
	}

	for _, r := range []resource.Resource{affiliate1, affiliate2, affiliate3} {
		suite.Require().NoError(suite.state.Create(suite.ctx, r))
	}

	// affiliates with non-empty Nodename should be translated to Members
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewMember(cluster.NamespaceName, affiliate1.TypedSpec().Nodename).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Member).TypedSpec()

			suite.Assert().Equal(affiliate1.TypedSpec().NodeID, spec.NodeID)
			suite.Assert().Equal([]netaddr.IP{netaddr.MustParseIP("192.168.3.4")}, spec.Addresses)
			suite.Assert().Equal("foo.com", spec.Hostname)
			suite.Assert().Equal(machine.TypeControlPlane, spec.MachineType)
			suite.Assert().Equal("Talos (v1.0.0)", spec.OperatingSystem)

			return nil
		}),
	))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewMember(cluster.NamespaceName, affiliate2.TypedSpec().Nodename).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Member).TypedSpec()

			suite.Assert().Equal(affiliate2.TypedSpec().NodeID, spec.NodeID)
			suite.Assert().Equal([]netaddr.IP{netaddr.MustParseIP("192.168.3.5")}, spec.Addresses)
			suite.Assert().Equal("worker-1", spec.Hostname)
			suite.Assert().Equal(machine.TypeWorker, spec.MachineType)

			return nil
		}),
	))

	// remove affiliate2, member information should eventually go away
	suite.Require().NoError(suite.state.Destroy(suite.ctx, affiliate2.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(*cluster.NewMember(cluster.NamespaceName, affiliate2.TypedSpec().Nodename).Metadata()),
	))
}

func TestMemberSuite(t *testing.T) {
	suite.Run(t, new(MemberSuite))
}
