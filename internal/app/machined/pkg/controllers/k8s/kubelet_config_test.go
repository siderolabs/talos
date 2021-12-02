// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"log"
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
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

type KubeletConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *KubeletConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.KubeletConfigController{}))

	suite.startRuntime()
}

func (suite *KubeletConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *KubeletConfigSuite) TestReconcile() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineKubelet: &v1alpha1.KubeletConfig{
				KubeletImage:      "kubelet",
				KubeletClusterDNS: []string{"10.0.0.1"},
				KubeletExtraArgs: map[string]string{
					"enable-feature": "foo",
				},
				KubeletExtraMounts: []v1alpha1.ExtraMount{
					{
						Mount: specs.Mount{
							Destination: "/tmp",
							Source:      "/var",
							Type:        "tmpfs",
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
			ExternalCloudProviderConfig: &v1alpha1.ExternalCloudProviderConfig{
				ExternalEnabled: true,
			},
			ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
				DNSDomain: "service.svc",
			},
		},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			kubeletConfig, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletConfigType, k8s.KubeletID, resource.VersionUndefined))
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
				map[string]string{
					"enable-feature": "foo",
				},
				spec.ExtraArgs)
			suite.Assert().Equal(
				[]specs.Mount{
					{
						Destination: "/tmp",
						Source:      "/var",
						Type:        "tmpfs",
					},
				},
				spec.ExtraMounts)
			suite.Assert().True(spec.CloudProviderExternal)

			return nil
		},
	))
}

func (suite *KubeletConfigSuite) TestReconcileDefaults() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
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
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			kubeletConfig, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletConfigType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := kubeletConfig.(*k8s.KubeletConfig).TypedSpec()

			suite.Assert().Equal("kubelet", spec.Image)
			suite.Assert().Equal([]string{"10.96.0.10"}, spec.ClusterDNS)
			suite.Assert().Equal("", spec.ClusterDomain)
			suite.Assert().Empty(spec.ExtraArgs)
			suite.Assert().Empty(spec.ExtraMounts)
			suite.Assert().False(spec.CloudProviderExternal)

			return nil
		},
	))
}

func (suite *KubeletConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKubeletConfigSuite(t *testing.T) {
	suite.Run(t, new(KubeletConfigSuite))
}
