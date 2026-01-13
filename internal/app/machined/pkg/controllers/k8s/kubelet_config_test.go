// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubeletConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *KubeletConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(k8sctrl.NewKubeletConfigController()))

	suite.startRuntime()
}

func (suite *KubeletConfigSuite) startRuntime() {
	suite.wg.Go(func() {
		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	})
}

func (suite *KubeletConfigSuite) createStaticPodServerStatus() {
	staticPodServerStatus := k8s.NewStaticPodServerStatus(k8s.NamespaceName, k8s.StaticPodServerStatusResourceID)

	staticPodServerStatus.TypedSpec().URL = "http://127.0.0.1:12345"

	suite.Require().NoError(suite.state.Create(suite.ctx, staticPodServerStatus))
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
						KubeletDefaultRuntimeSeccompProfileEnabled: pointer.To(true),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
						ExternalEnabled: pointer.To(true),
					},
					ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
						DNSDomain: "service.svc",
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				kubeletConfig, err := suite.state.Get(
					suite.ctx,
					resource.NewMetadata(
						k8s.NamespaceName,
						k8s.KubeletConfigType,
						k8s.KubeletID,
						resource.VersionUndefined,
					),
				)
				if err != nil {
					if state.IsNotFoundError(err) {
						return retry.ExpectedError(err)
					}

					return err
				}

				spec := kubeletConfig.(*k8s.KubeletConfig).TypedSpec()

				suite.Assert().Equal("kubelet", spec.Image)
				suite.Assert().Equal([]string{"10.0.0.1"}, spec.ClusterDNS)
				suite.Assert().Equal("service.svc", spec.ClusterDomain)
				suite.Assert().Equal(
					map[string]k8s.ArgValues{
						"enable-feature": {Values: []string{"foo"}},
					},
					spec.ExtraArgs,
				)
				suite.Assert().Equal(
					[]specs.Mount{
						{
							Destination: "/tmp",
							Source:      "/var",
							Type:        "tmpfs",
						},
					},
					spec.ExtraMounts,
				)
				suite.Assert().Equal(
					map[string]any{
						"serverTLSBootstrap": true,
					},
					spec.ExtraConfig,
				)
				suite.Assert().True(spec.CloudProviderExternal)
				suite.Assert().True(spec.DefaultRuntimeSeccompEnabled)

				return nil
			},
		),
	)
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

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				kubeletConfig, err := suite.state.Get(
					suite.ctx,
					resource.NewMetadata(
						k8s.NamespaceName,
						k8s.KubeletConfigType,
						k8s.KubeletID,
						resource.VersionUndefined,
					),
				)
				if err != nil {
					if state.IsNotFoundError(err) {
						return retry.ExpectedError(err)
					}

					return err
				}

				spec := kubeletConfig.(*k8s.KubeletConfig).TypedSpec()

				suite.Assert().Equal("kubelet", spec.Image)
				suite.Assert().Equal([]string{"10.96.0.10"}, spec.ClusterDNS)
				suite.Assert().Equal(constants.DefaultDNSDomain, spec.ClusterDomain)
				suite.Assert().Empty(spec.ExtraArgs)
				suite.Assert().Empty(spec.ExtraMounts)
				suite.Assert().False(spec.CloudProviderExternal)

				return nil
			},
		),
	)
}

func (suite *KubeletConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKubeletConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(KubeletConfigSuite))
}
