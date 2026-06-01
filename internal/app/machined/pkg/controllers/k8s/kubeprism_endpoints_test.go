// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	clusteradapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubePrismControllerSuite struct {
	ctest.DefaultSuite
}

func (suite *KubePrismControllerSuite) TestGeneration() {
	nodeIdentity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	suite.Require().NoError(clusteradapter.IdentitySpec(nodeIdentity.TypedSpec()).Generate())
	suite.Create(nodeIdentity)

	mc := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "controlplane",
		},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: must(url.Parse("https://example.com"))(suite.Require()),
				},
				LocalAPIServerPort: 6445,
			},
		},
	}))

	suite.Create(mc)

	member1 := cluster.NewMember(cluster.NamespaceName, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC")
	*member1.TypedSpec() = cluster.MemberSpec{
		NodeID:       "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Hostname:     "foo.com",
		MachineType:  machine.TypeControlPlane,
		Addresses:    []netip.Addr{netip.MustParseAddr("192.168.3.4")},
		ControlPlane: &cluster.ControlPlane{APIServerPort: 6446},
	}

	suite.Create(member1)

	member2 := cluster.NewMember(cluster.NamespaceName, "service/xCnFFfxylOf9i5ynhAkt6ZbfcqaLDGKfIa3gwpuaxe7F")
	*member2.TypedSpec() = cluster.MemberSpec{
		NodeID:       nodeIdentity.TypedSpec().NodeID,
		Hostname:     "foo2.com",
		MachineType:  machine.TypeControlPlane,
		Addresses:    []netip.Addr{netip.MustParseAddr("192.168.3.6")},
		ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
	}

	suite.Create(member2)

	member3 := cluster.NewMember(cluster.NamespaceName, "service/9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F")
	*member3.TypedSpec() = cluster.MemberSpec{
		NodeID:      "9dwHNUViZlPlIervqX9Qo256RUhrfhgO0xBBnKcKl4F",
		Hostname:    "worker-1",
		MachineType: machine.TypeWorker,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.5")},
	}

	suite.Create(member3)

	ctest.AssertResource(suite, k8s.KubePrismEndpointsID, func(e *k8s.KubePrismEndpoints, asrt *assert.Assertions) {
		asrt.Equal(
			&k8s.KubePrismEndpointsSpec{
				Endpoints: []k8s.KubePrismEndpoint{
					{
						Host: "example.com",
						Port: 443,
					},
					{
						Host: "localhost",
						Port: 6445,
					},
					{
						Host: "192.168.3.4",
						Port: 6446,
					},
					{
						Host: "192.168.3.6",
						Port: 6443,
					},
				},
			},
			e.TypedSpec(),
		)
	})
}

func must[T any](res T, err error) func(t *require.Assertions) T {
	return func(t *require.Assertions) T {
		t.NoError(err)

		return res
	}
}

func TestEndpointsBalancerControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KubePrismControllerSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(clusterctrl.NewKubePrismEndpointsController()))
			},
		},
	})
}
