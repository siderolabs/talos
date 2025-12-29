// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	kubespanctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

type ConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ConfigSuite) TestReconcileConfig() {
	ctr, err := container.New(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
					NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{ //nolint:staticcheck // legacy config
						KubeSpanEnabled: pointer.To(true),
					},
				},
			},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ClusterID:     "8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=",
				ClusterSecret: "I+1In7fLnpcRIjUmEoeugZnSyFoTF6MztLxICL5Yu0s=",
			},
		},
		&network.KubespanEndpointsConfigV1Alpha1{
			ExtraAnnouncedEndpointsConfig: []netip.AddrPort{
				netip.MustParseAddrPort("192.168.33.11:1001"),
			},
		},
	)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResource(suite, kubespan.ConfigID, func(res *kubespan.Config, asrt *assert.Assertions) {
		spec := res.TypedSpec()

		asrt.True(spec.Enabled)
		asrt.Equal("8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=", spec.ClusterID)
		asrt.Equal("I+1In7fLnpcRIjUmEoeugZnSyFoTF6MztLxICL5Yu0s=", spec.SharedSecret)
		asrt.True(spec.ForceRouting)
		asrt.False(spec.AdvertiseKubernetesNetworks)
		asrt.False(spec.HarvestExtraEndpoints)
		asrt.Equal("[\"192.168.33.11:1001\"]", fmt.Sprintf("%q", spec.ExtraEndpoints))
	})
}

func (suite *ConfigSuite) TestReconcileDisabled() {
	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			}))
	suite.Create(cfg)

	ctest.AssertResource(suite, kubespan.ConfigID, func(res *kubespan.Config, asrt *assert.Assertions) {
		spec := res.TypedSpec()

		asrt.False(spec.Enabled)
	})
}

func (suite *ConfigSuite) TestReconcileMultiDoc() {
	kubeSpanCfg := network.NewKubeSpanV1Alpha1()
	kubeSpanCfg.ConfigEnabled = pointer.To(true)
	kubeSpanCfg.ConfigMTU = pointer.To(uint32(1380))
	kubeSpanCfg.ConfigFilters = &network.KubeSpanFiltersConfig{
		ConfigEndpoints: []string{"0.0.0.0/0", "::/0"},
	}

	ctr, err := container.New(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ClusterID:     "test-cluster-id-multi-doc",
				ClusterSecret: "test-cluster-secret-multi-doc",
			},
		},
		kubeSpanCfg,
	)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResource(suite, kubespan.ConfigID,
		func(res *kubespan.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.True(spec.Enabled)
			asrt.Equal("test-cluster-id-multi-doc", spec.ClusterID)
			asrt.Equal("test-cluster-secret-multi-doc", spec.SharedSecret)
			asrt.Equal(uint32(1380), spec.MTU)
			asrt.Equal([]string{"0.0.0.0/0", "::/0"}, spec.EndpointFilters)
		},
	)
}

func TestConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(kubespanctrl.NewConfigController()))
			},
		},
	})
}
