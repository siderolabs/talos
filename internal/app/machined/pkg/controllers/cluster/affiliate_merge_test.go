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

type AffiliateMergeSuite struct {
	ClusterSuite
}

func (suite *AffiliateMergeSuite) TestReconcileDefault() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.AffiliateMergeController{}))

	affiliate1 := cluster.NewAffiliate(cluster.RawNamespaceName, "k8s/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
	*affiliate1.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Hostname:    "foo.com",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netaddr.IP{netaddr.MustParseIP("192.168.3.4")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
			Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
			AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
			Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")},
		},
	}

	affiliate2 := cluster.NewAffiliate(cluster.RawNamespaceName, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
	*affiliate2.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Hostname:    "foo.com",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netaddr.IP{netaddr.MustParseIP("192.168.3.4"), netaddr.MustParseIP("10.5.0.2")},
	}

	affiliate3 := cluster.NewAffiliate(cluster.RawNamespaceName, "service/9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F")
	*affiliate3.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F",
		Hostname:    "worker-1",
		Nodename:    "worker-1",
		MachineType: machine.TypeWorker,
		Addresses:   []netaddr.IP{netaddr.MustParseIP("192.168.3.5")},
	}

	for _, r := range []resource.Resource{affiliate1, affiliate2, affiliate3} {
		suite.Require().NoError(suite.state.Create(suite.ctx, r))
	}

	// there should be two merged affiliates: one from affiliate1+affiliate2, and another from affiliate3
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.NamespaceName, affiliate1.TypedSpec().NodeID).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			suite.Assert().Equal(affiliate1.TypedSpec().NodeID, spec.NodeID)
			suite.Assert().Equal([]netaddr.IP{netaddr.MustParseIP("192.168.3.4"), netaddr.MustParseIP("10.5.0.2")}, spec.Addresses)
			suite.Assert().Equal("foo.com", spec.Hostname)
			suite.Assert().Equal("bar", spec.Nodename)
			suite.Assert().Equal(machine.TypeControlPlane, spec.MachineType)
			suite.Assert().Equal(netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"), spec.KubeSpan.Address)
			suite.Assert().Equal("PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=", spec.KubeSpan.PublicKey)
			suite.Assert().Equal([]netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")}, spec.KubeSpan.AdditionalAddresses)
			suite.Assert().Equal([]netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")}, spec.KubeSpan.Endpoints)

			return nil
		}),
	))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.NamespaceName, affiliate3.TypedSpec().NodeID).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			suite.Assert().Equal(affiliate3.TypedSpec().NodeID, spec.NodeID)
			suite.Assert().Equal([]netaddr.IP{netaddr.MustParseIP("192.168.3.5")}, spec.Addresses)
			suite.Assert().Equal("worker-1", spec.Hostname)
			suite.Assert().Equal("worker-1", spec.Nodename)
			suite.Assert().Equal(machine.TypeWorker, spec.MachineType)
			suite.Assert().Zero(spec.KubeSpan.PublicKey)

			return nil
		}),
	))

	// remove affiliate2, KubeSpan information should eventually go away
	suite.Require().NoError(suite.state.Destroy(suite.ctx, affiliate1.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.NamespaceName, affiliate1.TypedSpec().NodeID).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			suite.Assert().Equal(affiliate1.TypedSpec().NodeID, spec.NodeID)

			if spec.KubeSpan.PublicKey != "" {
				return retry.ExpectedErrorf("not reconciled yet")
			}

			suite.Assert().Zero(spec.KubeSpan.Address)
			suite.Assert().Zero(spec.KubeSpan.PublicKey)
			suite.Assert().Zero(spec.KubeSpan.AdditionalAddresses)
			suite.Assert().Zero(spec.KubeSpan.Endpoints)

			return nil
		}),
	))

	// remove affiliate3, merged affiliate should be removed
	suite.Require().NoError(suite.state.Destroy(suite.ctx, affiliate3.Metadata()))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(*cluster.NewAffiliate(cluster.NamespaceName, affiliate3.TypedSpec().NodeID).Metadata()),
	))
}

func TestAffiliateMergeSuite(t *testing.T) {
	suite.Run(t, new(AffiliateMergeSuite))
}
