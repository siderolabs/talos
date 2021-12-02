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
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

type NodeIPConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *NodeIPConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.NodeIPConfigController{}))

	suite.startRuntime()
}

func (suite *NodeIPConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *NodeIPConfigSuite) TestReconcileWithSubnets() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineKubelet: &v1alpha1.KubeletConfig{
				KubeletNodeIP: v1alpha1.KubeletNodeIPConfig{
					KubeletNodeIPValidSubnets: []string{"10.0.0.0/24"},
				},
			},
			MachineNetwork: &v1alpha1.NetworkConfig{
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
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			NodeIPConfig, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodeIPConfigType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := NodeIPConfig.(*k8s.NodeIPConfig).TypedSpec()

			suite.Assert().Equal([]string{"10.0.0.0/24"}, spec.ValidSubnets)
			suite.Assert().Equal([]string{"1.2.3.4", "5.6.7.8"}, spec.ExcludeSubnets)

			return nil
		},
	))
}

func (suite *NodeIPConfigSuite) TestReconcileDefaults() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
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
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			NodeIPConfig, err := suite.state.Get(suite.ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodeIPConfigType, k8s.KubeletID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			spec := NodeIPConfig.(*k8s.NodeIPConfig).TypedSpec()

			suite.Assert().Equal([]string{"0.0.0.0/0", "::/0"}, spec.ValidSubnets)
			suite.Assert().Empty(spec.ExcludeSubnets)

			return nil
		},
	))
}

func (suite *NodeIPConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestNodeIPConfigSuite(t *testing.T) {
	suite.Run(t, new(NodeIPConfigSuite))
}
