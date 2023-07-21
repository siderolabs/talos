// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net/netip"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/slices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type EndpointSuite struct {
	ClusterSuite
}

func (suite *EndpointSuite) TestReconcileDefault() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.EndpointController{}))

	member1 := cluster.NewMember(cluster.NamespaceName, "talos-default-controlplane-1")
	*member1.TypedSpec() = cluster.MemberSpec{
		NodeID:          "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Addresses:       []netip.Addr{netip.MustParseAddr("172.20.0.2"), netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0")},
		Hostname:        "talos-default-controlplane-1",
		MachineType:     machine.TypeControlPlane,
		OperatingSystem: "Talos (v1.0.0)",
	}

	member2 := cluster.NewMember(cluster.NamespaceName, "talos-default-controlplane-2")
	*member2.TypedSpec() = cluster.MemberSpec{
		NodeID:          "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F",
		Addresses:       []netip.Addr{netip.MustParseAddr("172.20.0.3"), netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e1")},
		Hostname:        "talos-default-controlplane-2",
		MachineType:     machine.TypeControlPlane,
		OperatingSystem: "Talos (v1.0.0)",
	}

	member3 := cluster.NewMember(cluster.NamespaceName, "talos-default-worker-1")
	*member3.TypedSpec() = cluster.MemberSpec{
		NodeID:          "xCnFFfxylOf9i5ynhAkt6ZbfcqaLDGKfIa3gwpuaxe7F",
		Addresses:       []netip.Addr{netip.MustParseAddr("172.20.0.4")},
		Hostname:        "talos-default-worker-1",
		MachineType:     machine.TypeWorker,
		OperatingSystem: "Talos (v1.0.0)",
	}

	for _, r := range []resource.Resource{member1, member2, member3} {
		suite.Require().NoError(suite.state.Create(suite.ctx, r))
	}

	// control plane members should be translated to Endpoints
	ctest.AssertResource(suite, k8s.ControlPlaneDiscoveredEndpointsID, func(r *k8s.Endpoint, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(
			[]string{
				"172.20.0.2",
				"172.20.0.3",
				"fd50:8d60:4238:6302:f857:23ff:fe21:d1e0",
				"fd50:8d60:4238:6302:f857:23ff:fe21:d1e1",
			},
			slices.Map(spec.Addresses, netip.Addr.String),
		)
	})
}

func TestEndpointSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(EndpointSuite))
}
