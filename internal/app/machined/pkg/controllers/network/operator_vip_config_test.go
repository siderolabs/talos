// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
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
	"inet.af/netaddr"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
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
	check func(*network.OperatorSpec) error,
) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.OperatorSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *OperatorVIPConfigSuite) TestMachineConfigurationVIP() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorVIPConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineNetwork: &v1alpha1.NetworkConfig{
					NetworkInterfaces: []*v1alpha1.Device{
						{
							DeviceInterface: "eth1",
							DeviceDHCP:      true,
							DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
								SharedIP: "2.3.4.5",
							},
						},
						{
							DeviceInterface: "eth2",
							DeviceDHCP:      true,
							DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
								SharedIP: "fd7a:115c:a1e0:ab12:4843:cd96:6277:2302",
							},
						},
						{
							DeviceInterface: "eth3",
							DeviceDHCP:      true,
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
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertOperators(
					[]string{
						"configuration/vip/eth1",
						"configuration/vip/eth2",
						"configuration/vip/eth3.26",
					}, func(r *network.OperatorSpec) error {
						suite.Assert().Equal(network.OperatorVIP, r.TypedSpec().Operator)
						suite.Assert().True(r.TypedSpec().RequireUp)

						switch r.Metadata().ID() {
						case "configuration/vip/eth1":
							suite.Assert().Equal("eth1", r.TypedSpec().LinkName)
							suite.Assert().EqualValues(netaddr.MustParseIP("2.3.4.5"), r.TypedSpec().VIP.IP)
						case "configuration/vip/eth2":
							suite.Assert().Equal("eth2", r.TypedSpec().LinkName)
							suite.Assert().EqualValues(
								netaddr.MustParseIP("fd7a:115c:a1e0:ab12:4843:cd96:6277:2302"),
								r.TypedSpec().VIP.IP,
							)
						case "configuration/vip/eth3.26":
							suite.Assert().Equal("eth3.26", r.TypedSpec().LinkName)
							suite.Assert().EqualValues(netaddr.MustParseIP("5.5.4.4"), r.TypedSpec().VIP.IP)
						}

						return nil
					},
				)
			},
		),
	)
}

func (suite *OperatorVIPConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(
		context.Background(), config.NewMachineConfig(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
			},
		),
	)
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Require().NoError(err)

	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewLinkStatus(network.ConfigNamespaceName, "bar"),
		),
	)
}

func TestOperatorVIPConfigSuite(t *testing.T) {
	suite.Run(t, new(OperatorVIPConfigSuite))
}
