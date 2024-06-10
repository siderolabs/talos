// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type AddressConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *AddressConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.DeviceConfigController{}))
}

func (suite *AddressConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *AddressConfigSuite) assertAddresses(requiredIDs []string, check func(*network.AddressSpec, *assert.Assertions)) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *AddressConfigSuite) TestLoopback() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressConfigController{}))

	suite.startRuntime()

	suite.assertAddresses(
		[]string{
			"default/lo/127.0.0.1/8",
		}, func(r *network.AddressSpec, asrt *assert.Assertions) {
			asrt.Equal("lo", r.TypedSpec().LinkName)
			asrt.Equal(nethelpers.ScopeHost, r.TypedSpec().Scope)
		},
	)
}

func (suite *AddressConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.AddressConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1::::: ip=eth3:dhcp ip=10.3.5.7::10.3.5.1:255.255.255.0::eth4"),
			},
		),
	)

	suite.startRuntime()

	suite.assertAddresses(
		[]string{
			"cmdline/eth1/172.20.0.2/24",
			"cmdline/eth4/10.3.5.7/24",
		}, func(r *network.AddressSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "cmdline/eth1/172.20.0.2/24":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
			case "cmdline/eth4/10.3.5.7/24":
				asrt.Equal("eth4", r.TypedSpec().LinkName)
			}
		},
	)
}

func (suite *AddressConfigSuite) TestCmdlineNoNetmask() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.AddressConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1"),
			},
		),
	)

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

	suite.assertAddresses(
		[]string{
			fmt.Sprintf("cmdline/%s/172.20.0.2/32", ifaceName),
		}, func(r *network.AddressSpec, asrt *assert.Assertions) {
			asrt.Equal(ifaceName, r.TypedSpec().LinkName)
			asrt.Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)
		},
	)
}

func (suite *AddressConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.AddressConfigController{}))

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
								DeviceInterface: "eth3",
								DeviceCIDR:      "192.168.0.24/28",
							},
							{
								DeviceIgnore:    pointer.To(true),
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
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.assertAddresses(
		[]string{
			"configuration/eth2/2001:470:6d:30e:8ed2:b60c:9d2f:803a/64",
			"configuration/eth3/192.168.0.24/28",
			"configuration/eth5/10.5.0.7/32",
			"configuration/eth6/10.5.0.8/32",
			"configuration/eth6/2001:470:6d:30e:8ed2:b60c:9d2f:803b/64",
			"configuration/eth0.24/10.0.0.1/8",
			"configuration/eth0.25/11.0.0.1/8",
		}, func(r *network.AddressSpec, asrt *assert.Assertions) {},
	)
}

func (suite *AddressConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestAddressConfigSuite(t *testing.T) {
	suite.Run(t, new(AddressConfigSuite))
}
