// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type NodeIPConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *NodeIPConfigSuite) TestReconcileWithSubnets() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletNodeIP: &v1alpha1.KubeletNodeIPConfig{
							KubeletNodeIPValidSubnets: []string{"10.0.0.0/24"},
						},
					},
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy controller
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
									SharedIP: "1.2.3.4",
								},
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID: 100,
										VlanVIP: &v1alpha1.DeviceVIPConfig{
											SharedIP: "5.6.7.8",
										},
									},
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						ServiceSubnet: []string{constants.DefaultIPv4ServiceNet},
						PodSubnet:     []string{constants.DefaultIPv4PodNet},
					},
				},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, k8s.KubeletID, func(cfg *k8s.NodeIPConfig, asrt *assert.Assertions) {
		spec := cfg.TypedSpec()

		asrt.Equal([]string{"10.0.0.0/24"}, spec.ValidSubnets)
		asrt.Equal(
			[]string{"10.244.0.0/16", "10.96.0.0/12", "1.2.3.4", "5.6.7.8"},
			spec.ExcludeSubnets,
		)
	})
}

func (suite *NodeIPConfigSuite) TestReconcileWithNewVIPs() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfgV1Alpha1 := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: u,
				},
			},
			ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
				ServiceSubnet: []string{constants.DefaultIPv4ServiceNet},
				PodSubnet:     []string{constants.DefaultIPv4PodNet},
			},
		},
	}

	cfgVIP := network.NewLayer2VIPConfigV1Alpha1("5.6.7.8")
	cfgVIP.LinkName = "eth0"

	ctr, err := container.New(cfgV1Alpha1, cfgVIP)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)

	suite.Create(cfg)

	ctest.AssertResource(suite, k8s.KubeletID, func(cfg *k8s.NodeIPConfig, asrt *assert.Assertions) {
		spec := cfg.TypedSpec()

		asrt.Equal([]string{"0.0.0.0/0"}, spec.ValidSubnets)
		asrt.Equal(
			[]string{"10.244.0.0/16", "10.96.0.0/12", "5.6.7.8"},
			spec.ExcludeSubnets,
		)
	})
}

func (suite *NodeIPConfigSuite) TestReconcileDefaults() {
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
						ServiceSubnet: []string{constants.DefaultIPv4ServiceNet, constants.DefaultIPv6ServiceNet},
						PodSubnet:     []string{constants.DefaultIPv4PodNet, constants.DefaultIPv6PodNet},
					},
				},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, k8s.KubeletID, func(cfg *k8s.NodeIPConfig, asrt *assert.Assertions) {
		spec := cfg.TypedSpec()

		asrt.Equal([]string{"0.0.0.0/0", "::/0"}, spec.ValidSubnets)
		asrt.Equal(
			[]string{"10.244.0.0/16", "fc00:db8:10::/56", "10.96.0.0/12", "fc00:db8:20::/112"},
			spec.ExcludeSubnets,
		)
	})
}

func TestNodeIPConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &NodeIPConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewNodeIPConfigController()))
			},
		},
	})
}
