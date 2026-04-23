// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubeletConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *KubeletConfigSuite) createStaticPodServerStatus() {
	staticPodServerStatus := k8s.NewStaticPodServerStatus(k8s.NamespaceName, k8s.StaticPodServerStatusResourceID)

	staticPodServerStatus.TypedSpec().URL = "http://127.0.0.1:12345"

	suite.Create(staticPodServerStatus)
}

func (suite *KubeletConfigSuite) TestReconcile() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	suite.createStaticPodServerStatus()

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletImage:      "kubelet",
						KubeletClusterDNS: []string{"10.0.0.1"},
						KubeletExtraArgs: v1alpha1.Args{
							"enable-feature": v1alpha1.NewArgValue("foo", nil),
						},
						KubeletExtraMounts: []v1alpha1.ExtraMount{
							{
								Destination: "/tmp",
								Source:      "/var",
								Type:        "tmpfs",
							},
						},
						KubeletExtraConfig: v1alpha1.Unstructured{
							Object: map[string]any{
								"serverTLSBootstrap": true,
							},
						},
						KubeletDefaultRuntimeSeccompProfileEnabled: new(true),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: new(true),
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						DNSDomain: "service.svc",
					},
				},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, k8s.KubeletID, func(r *k8s.KubeletConfig, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("kubelet", spec.Image)
		asrt.Equal([]string{"10.0.0.1"}, spec.ClusterDNS)
		asrt.Equal("service.svc", spec.ClusterDomain)
		asrt.Equal(
			map[string]k8s.ArgValues{
				"enable-feature": {Values: []string{"foo"}},
			},
			spec.ExtraArgs,
		)
		asrt.Equal(
			[]specs.Mount{
				{
					Destination: "/tmp",
					Source:      "/var",
					Type:        "tmpfs",
				},
			},
			spec.ExtraMounts,
		)
		asrt.Equal(
			map[string]any{
				"serverTLSBootstrap": true,
			},
			spec.ExtraConfig,
		)
		asrt.True(spec.CloudProviderExternal)
		asrt.True(spec.DefaultRuntimeSeccompEnabled)
	})
}

func (suite *KubeletConfigSuite) TestReconcileDefaults() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	suite.createStaticPodServerStatus()

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKubelet: &v1alpha1.KubeletConfig{
						KubeletImage: "kubelet",
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
					},
				},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, k8s.KubeletID, func(r *k8s.KubeletConfig, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("kubelet", spec.Image)
		asrt.Equal([]string{"10.96.0.10"}, spec.ClusterDNS)
		asrt.Equal(constants.DefaultDNSDomain, spec.ClusterDomain)
		asrt.Empty(spec.ExtraArgs)
		asrt.Empty(spec.ExtraMounts)
		asrt.False(spec.CloudProviderExternal)
	})
}

func TestKubeletConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KubeletConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(k8sctrl.NewKubeletConfigController()))
			},
		},
	})
}
