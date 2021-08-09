// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"sort"
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
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/network"
)

type AddressConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *AddressConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)
}

func (suite *AddressConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *AddressConfigSuite) assertAddresses(requiredIDs []string, check func(*network.AddressSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.AddressSpecType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.AddressSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *AddressConfigSuite) TestLoopback() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressConfigController{}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertAddresses([]string{
				"default/lo/127.0.0.1/8",
			}, func(r *network.AddressSpec) error {
				suite.Assert().Equal("lo", r.TypedSpec().LinkName)
				suite.Assert().Equal(nethelpers.ScopeHost, r.TypedSpec().Scope)

				return nil
			})
		}))
}

func (suite *AddressConfigSuite) TestCmdline() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressConfigController{
		Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1:::::"),
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertAddresses([]string{
				"cmdline/eth1/172.20.0.2/24",
			}, func(r *network.AddressSpec) error {
				suite.Assert().Equal("eth1", r.TypedSpec().LinkName)

				return nil
			})
		}))
}

func (suite *AddressConfigSuite) TestCmdlineNoNetmask() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressConfigController{
		Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1"),
	}))

	suite.startRuntime()

	ifaces, _ := net.Interfaces() //nolint:errcheck // ignoring error here as ifaces will be empty

	sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })

	ifaceName := ""

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		ifaceName = iface.Name

		break
	}

	suite.Assert().NotEmpty(ifaceName)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertAddresses([]string{
				fmt.Sprintf("cmdline/%s/172.20.0.2/32", ifaceName),
			}, func(r *network.AddressSpec) error {
				suite.Assert().Equal(ifaceName, r.TypedSpec().LinkName)
				suite.Assert().Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)

				return nil
			})
		}))
}

func (suite *AddressConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressConfigController{}))

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
					},
					{
						DeviceIgnore:    true,
						DeviceInterface: "eth4",
						DeviceCIDR:      "192.168.0.24/28",
					},
					{
						DeviceInterface: "eth2",
						DeviceCIDR:      "2001:470:6d:30e:8ed2:b60c:9d2f:803a/64",
					},
					{
						DeviceInterface: "eth5",
						DeviceCIDR:      "10.5.0.7",
					},
					{
						DeviceInterface: "eth6",
						DeviceAddresses: []string{
							"10.5.0.8",
							"2001:470:6d:30e:8ed2:b60c:9d2f:803b/64",
						},
					},
					{
						DeviceInterface: "eth0",
						DeviceVlans: []*v1alpha1.Vlan{
							{
								VlanID:   24,
								VlanCIDR: "10.0.0.1/8",
							},
							{
								VlanID: 25,
								VlanAddresses: []string{
									"11.0.0.1/8",
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
			return suite.assertAddresses([]string{
				"configuration/eth2/2001:470:6d:30e:8ed2:b60c:9d2f:803a/64",
				"configuration/eth3/192.168.0.24/28",
				"configuration/eth5/10.5.0.7/32",
				"configuration/eth6/10.5.0.8/32",
				"configuration/eth6/2001:470:6d:30e:8ed2:b60c:9d2f:803b/64",
				"configuration/eth0.24/10.0.0.1/8",
				"configuration/eth0.25/11.0.0.1/8",
			}, func(r *network.AddressSpec) error {
				return nil
			})
		}))
}

func (suite *AddressConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(context.Background(), config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
	}))
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Require().NoError(err)
}

func TestAddressConfigSuite(t *testing.T) {
	suite.Run(t, new(AddressConfigSuite))
}
