// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"net/netip"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type OperatorVIPConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *OperatorVIPConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.DeviceConfigController{}))
}

func (suite *OperatorVIPConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *OperatorVIPConfigSuite) assertOperators(
	requiredIDs []string,
	check func(*network.OperatorSpec, *assert.Assertions),
) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *OperatorVIPConfigSuite) TestMachineConfigurationVIP() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorVIPConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth1",
								DeviceDHCP:      pointer.To(true),
								DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
									SharedIP: "2.3.4.5",
								},
							},
							{
								DeviceInterface: "eth2",
								DeviceDHCP:      pointer.To(true),
								DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
									SharedIP: "fd7a:115c:a1e0:ab12:4843:cd96:6277:2302",
								},
							},
							{
								DeviceInterface: "eth3",
								DeviceDHCP:      pointer.To(true),
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID: 26,
										VlanVIP: &v1alpha1.DeviceVIPConfig{
											SharedIP: "5.5.4.4",
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
				},
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.assertOperators(
		[]string{
			"configuration/vip/eth1",
			"configuration/vip/eth2",
			"configuration/vip/eth3.26",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorVIP, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/vip/eth1":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
				asrt.EqualValues(netip.MustParseAddr("2.3.4.5"), r.TypedSpec().VIP.IP)
			case "configuration/vip/eth2":
				asrt.Equal("eth2", r.TypedSpec().LinkName)
				asrt.EqualValues(
					netip.MustParseAddr("fd7a:115c:a1e0:ab12:4843:cd96:6277:2302"),
					r.TypedSpec().VIP.IP,
				)
			case "configuration/vip/eth3.26":
				asrt.Equal("eth3.26", r.TypedSpec().LinkName)
				asrt.EqualValues(netip.MustParseAddr("5.5.4.4"), r.TypedSpec().VIP.IP)
			}
		},
	)
}

func (suite *OperatorVIPConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestOperatorVIPConfigSuite(t *testing.T) {
	suite.Run(t, new(OperatorVIPConfigSuite))
}
