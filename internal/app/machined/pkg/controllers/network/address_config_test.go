// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"cmp"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type AddressConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *AddressConfigSuite) TestLoopback() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.AddressConfigController{}))

	ctest.AssertResource(
		suite,
		"default/lo/127.0.0.1/8",
		func(r *network.AddressSpec, asrt *assert.Assertions) {
			asrt.Equal("lo", r.TypedSpec().LinkName)
			asrt.Equal(nethelpers.ScopeHost, r.TypedSpec().Scope)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *AddressConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.AddressConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1::::: ip=eth3:dhcp ip=10.3.5.7::10.3.5.1:255.255.255.0::eth4"),
			},
		),
	)

	ctest.AssertResources(
		suite,
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
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *AddressConfigSuite) TestCmdlineNoNetmask() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.AddressConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1"),
			},
		),
	)

	ifaces, _ := net.Interfaces() //nolint:errcheck // ignoring error here as ifaces will be empty

	slices.SortFunc(ifaces, func(a, b net.Interface) int { return cmp.Compare(a.Name, b.Name) })

	ifaceName := ""

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		ifaceName = iface.Name

		break
	}

	suite.Assert().NotEmpty(ifaceName)

	ctest.AssertResource(
		suite,
		fmt.Sprintf("cmdline/%s/172.20.0.2/32", ifaceName),
		func(r *network.AddressSpec, asrt *assert.Assertions) {
			asrt.Equal(ifaceName, r.TypedSpec().LinkName)
			asrt.Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *AddressConfigSuite) TestMachineConfigurationLegacy() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.AddressConfigController{}))
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DeviceConfigController{}))

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth3",
								DeviceCIDR:      "192.168.0.24/28",
							},
							{
								DeviceIgnore:    new(true),
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

	suite.Create(cfg)

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/eth2/2001:470:6d:30e:8ed2:b60c:9d2f:803a/64",
			"configuration/eth3/192.168.0.24/28",
			"configuration/eth5/10.5.0.7/32",
			"configuration/eth6/10.5.0.8/32",
			"configuration/eth6/2001:470:6d:30e:8ed2:b60c:9d2f:803b/64",
			"configuration/eth0.24/10.0.0.1/8",
			"configuration/eth0.25/11.0.0.1/8",
		},
		func(r *network.AddressSpec, asrt *assert.Assertions) {},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *AddressConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.AddressConfigController{}))

	lc1 := networkcfg.NewLinkConfigV1Alpha1("enp0s2")
	lc1.LinkAddresses = []networkcfg.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("10.12.3.4/24"),
		},
	}

	lc2 := networkcfg.NewLinkConfigV1Alpha1("enp0s3")
	lc2.LinkAddresses = []networkcfg.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("172.20.0.1/20"),
			AddressPriority: new(uint32(100)),
		},
	}

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/enp0s2/10.12.3.4/24",
			"configuration/enp0s3/172.20.0.1/20",
		},
		func(r *network.AddressSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			if r.Metadata().ID() == "configuration/enp0s3/172.20.0.1/20" {
				asrt.Equal(uint32(100), r.TypedSpec().Priority)
			}
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func TestAddressConfigSuite(t *testing.T) {
	suite.Run(t, &AddressConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
