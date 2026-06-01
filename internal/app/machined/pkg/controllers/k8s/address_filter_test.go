// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"fmt"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type K8sAddressFilterSuite struct {
	ctest.DefaultSuite
}

func (suite *K8sAddressFilterSuite) TestReconcile() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						ServiceSubnet: []string{
							"10.200.0.0/22",
							"fd40:10:200::/112",
						},
						PodSubnet: []string{
							"10.32.0.0/12",
							"fd00:10:32::/102",
						},
					},
				},
			},
		),
	)
	suite.Create(cfg)

	ctest.AssertResource(suite, k8s.NodeAddressFilterOnlyK8s, func(res *network.NodeAddressFilter, asrt *assert.Assertions) {
		spec := res.TypedSpec()

		asrt.Equal(
			"[10.32.0.0/12 fd00:10:32::/102 10.200.0.0/22 fd40:10:200::/112]",
			fmt.Sprintf("%s", spec.IncludeSubnets),
		)
		asrt.Empty(spec.ExcludeSubnets)
	})

	ctest.AssertResource(suite, k8s.NodeAddressFilterNoK8s, func(res *network.NodeAddressFilter, asrt *assert.Assertions) {
		spec := res.TypedSpec()

		asrt.Empty(spec.IncludeSubnets)
		asrt.Equal(
			"[10.32.0.0/12 fd00:10:32::/102 10.200.0.0/22 fd40:10:200::/112]",
			fmt.Sprintf("%s", spec.ExcludeSubnets),
		)
	})

	// create NodeStatus with PodCIDRs
	nodeName := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodeName.TypedSpec().Nodename = "test-node"
	suite.Create(nodeName)

	nodeStatus := k8s.NewNodeStatus(k8s.NamespaceName, "test-node")
	nodeStatus.TypedSpec().PodCIDRs = []netip.Prefix{
		netip.MustParsePrefix("192.168.0.0/24"),
	}
	suite.Create(nodeStatus)

	ctest.AssertResource(suite, k8s.NodeAddressFilterOnlyK8s, func(res *network.NodeAddressFilter, asrt *assert.Assertions) {
		spec := res.TypedSpec()

		asrt.Equal(
			"[10.32.0.0/12 fd00:10:32::/102 192.168.0.0/24 10.200.0.0/22 fd40:10:200::/112]",
			fmt.Sprintf("%s", spec.IncludeSubnets),
		)
		asrt.Empty(spec.ExcludeSubnets)
	})

	ctest.AssertResource(suite, k8s.NodeAddressFilterNoK8s, func(res *network.NodeAddressFilter, asrt *assert.Assertions) {
		spec := res.TypedSpec()

		asrt.Empty(spec.IncludeSubnets)
		asrt.Equal(
			"[10.32.0.0/12 fd00:10:32::/102 192.168.0.0/24 10.200.0.0/22 fd40:10:200::/112]",
			fmt.Sprintf("%s", spec.ExcludeSubnets),
		)
	})
}

func TestK8sAddressFilterSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &K8sAddressFilterSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.AddressFilterController{}))
			},
		},
	})
}
