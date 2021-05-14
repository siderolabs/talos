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
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/network"
	"github.com/talos-systems/talos/pkg/resources/network/nethelpers"
)

type RouteConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *RouteConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)
}

func (suite *RouteConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *RouteConfigSuite) assertRoutes(requiredIDs []string, check func(*network.RouteSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.RouteSpecType, "", resource.VersionUndefined))
	if err != nil {
		return retry.UnexpectedError(err)
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.RouteSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *RouteConfigSuite) TestCmdline() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.RouteConfigController{
		Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1:::::"),
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertRoutes([]string{
				"cmdline//172.20.0.1",
			}, func(r *network.RouteSpec) error {
				suite.Assert().Equal("eth1", r.Status().OutLinkName)
				suite.Assert().Equal(network.ConfigCmdline, r.Status().Layer)
				suite.Assert().Equal(nethelpers.FamilyInet4, r.Status().Family)
				suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.Status().Priority)

				return nil
			})
		}))
}

func (suite *RouteConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.RouteConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth3",
						DeviceCIDR:      "192.168.0.24/28",
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteNetwork: "192.168.0.0/18",
								RouteGateway: "192.168.0.25",
								RouteMetric:  25,
							},
						},
					},
					{
						DeviceIgnore:    true,
						DeviceInterface: "eth4",
						DeviceCIDR:      "192.168.0.24/28",
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteNetwork: "192.168.0.0/18",
								RouteGateway: "192.168.0.26",
								RouteMetric:  25,
							},
						},
					},
					{
						DeviceInterface: "eth2",
						DeviceCIDR:      "2001:470:6d:30e:8ed2:b60c:9d2f:803a/64",
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteGateway: "2001:470:6d:30e:8ed2:b60c:9d2f:803b",
							},
						},
					},
					{
						DeviceInterface: "eth0",
						DeviceVlans: []*v1alpha1.Vlan{
							{
								VlanID:   24,
								VlanCIDR: "10.0.0.1/8",
								VlanRoutes: []*v1alpha1.Route{
									{
										RouteNetwork: "10.0.3.0/24",
										RouteGateway: "10.0.3.1",
									},
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
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertRoutes([]string{
				"configuration//2001:470:6d:30e:8ed2:b60c:9d2f:803b",
				"configuration/10.0.3.0/24/10.0.3.1",
				"configuration/192.168.0.0/18/192.168.0.25",
			}, func(r *network.RouteSpec) error {
				switch r.Metadata().ID() {
				case "configuration//2001:470:6d:30e:8ed2:b60c:9d2f:803b":
					suite.Assert().Equal("eth2", r.Status().OutLinkName)
					suite.Assert().Equal(nethelpers.FamilyInet6, r.Status().Family)
					suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.Status().Priority)
				case "configuration/10.0.3.0/24/10.0.3.1":
					suite.Assert().Equal("eth0.24", r.Status().OutLinkName)
					suite.Assert().Equal(nethelpers.FamilyInet4, r.Status().Family)
					suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.Status().Priority)
				case "configuration/192.168.0.0/18/192.168.0.25":
					suite.Assert().Equal("eth3", r.Status().OutLinkName)
					suite.Assert().Equal(nethelpers.FamilyInet4, r.Status().Family)
					suite.Assert().EqualValues(25, r.Status().Priority)
				}

				suite.Assert().Equal(network.ConfigMachineConfiguration, r.Status().Layer)

				return nil
			})
		}))
}

func TestRouteConfigSuite(t *testing.T) {
	suite.Run(t, new(RouteConfigSuite))
}
