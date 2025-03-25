// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type OperatorConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *OperatorConfigSuite) assertOperators(requiredIDs []string, check func(*network.OperatorSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *OperatorConfigSuite) assertNoOperators(unexpectedIDs []string) {
	for _, id := range unexpectedIDs {
		ctest.AssertNoResource[*network.OperatorSpec](suite, id, rtestutils.WithNamespace(network.ConfigNamespaceName))
	}
}

func (suite *OperatorConfigSuite) TestDefaultDHCP() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.OperatorConfigController{
				Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth2"),
			},
		),
	)

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Create(linkStatus)
	}

	suite.assertOperators(
		[]string{
			"default/dhcp4/eth0",
			"default/dhcp4/eth1",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)
			asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)

			switch r.Metadata().ID() {
			case "default/dhcp4/eth0":
				asrt.Equal("eth0", r.TypedSpec().LinkName)
			case "default/dhcp4/eth1":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
			}
		},
	)
}

func (suite *OperatorConfigSuite) TestDefaultDHCPCmdline() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.OperatorConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1::::: ip=eth3:dhcp"),
			},
		),
	)

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Create(linkStatus)
	}

	suite.assertOperators(
		[]string{
			"default/dhcp4/eth0",
			"default/dhcp4/eth2",
			"cmdline/dhcp4/eth3",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)
			asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)

			switch r.Metadata().ID() {
			case "default/dhcp4/eth0":
				asrt.Equal("eth0", r.TypedSpec().LinkName)
			case "default/dhcp4/eth2":
				asrt.Equal("eth2", r.TypedSpec().LinkName)
			case "cmdline/dhcp4/eth3":
				asrt.Equal("eth3", r.TypedSpec().LinkName)
			}
		},
	)

	// remove link
	suite.Require().NoError(
		suite.State().Destroy(
			suite.Ctx(),
			resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "eth2", resource.VersionUndefined),
		),
	)

	suite.assertNoOperators(
		[]string{
			"default/dhcp4/eth2",
		},
	)
}

func (suite *OperatorConfigSuite) TestMachineConfigurationDHCP4() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.OperatorConfigController{
				Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth5"),
			},
		),
	)
	// add LinkConfig controller to produce link specs based on machine configuration
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.LinkConfigController{
				Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth5"),
			},
		),
	)

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Create(linkStatus)
	}

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
								DeviceInterface: "eth0",
							},
							{
								DeviceInterface: "eth1",
								DeviceDHCP:      pointer.To(true),
							},
							{
								DeviceIgnore:    pointer.To(true),
								DeviceInterface: "eth2",
								DeviceDHCP:      pointer.To(true),
							},
							{
								DeviceInterface: "eth3",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv4:        pointer.To(true),
									DHCPRouteMetric: 256,
								},
							},
							{
								DeviceInterface: "eth4",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID:   25,
										VlanDHCP: pointer.To(true),
									},
									{
										VlanID: 26,
									},
									{
										VlanID: 27,
										VlanDHCPOptions: &v1alpha1.DHCPOptions{
											DHCPRouteMetric: 256,
										},
									},
								},
							},
							{
								DeviceInterface: "eth5",
								DeviceDHCP:      pointer.To(true),
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

	suite.assertOperators(
		[]string{
			"configuration/dhcp4/eth1",
			"configuration/dhcp4/eth3",
			"configuration/dhcp4/eth4.25",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/dhcp4/eth1":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth3":
				asrt.Equal("eth3", r.TypedSpec().LinkName)
				asrt.EqualValues(256, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth4.25":
				asrt.Equal("eth4.25", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth4.26":
				asrt.Equal("eth4.26", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth4.27":
				asrt.Equal("eth4.27", r.TypedSpec().LinkName)
				asrt.EqualValues(256, r.TypedSpec().DHCP4.RouteMetric)
			}
		},
	)

	suite.assertNoOperators(
		[]string{
			"configuration/dhcp4/eth0",
			"default/dhcp4/eth0",
			"configuration/dhcp4/eth2",
			"default/dhcp4/eth2",
			"configuration/dhcp4/eth4.26",
		},
	)
}

func (suite *OperatorConfigSuite) TestMachineConfigurationDHCP6() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.OperatorConfigController{}))

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
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv4: pointer.To(true),
								},
							},
							{
								DeviceInterface: "eth2",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv6: pointer.To(true),
								},
							},
							{
								DeviceInterface: "eth3",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv6:        pointer.To(true),
									DHCPRouteMetric: 512,
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

	suite.assertOperators(
		[]string{
			"configuration/dhcp6/eth2",
			"configuration/dhcp6/eth3",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP6, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/dhcp6/eth2":
				asrt.Equal("eth2", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP6.RouteMetric)
			case "configuration/dhcp6/eth3":
				asrt.Equal("eth3", r.TypedSpec().LinkName)
				asrt.EqualValues(512, r.TypedSpec().DHCP6.RouteMetric)
			}
		},
	)

	suite.assertNoOperators(
		[]string{
			"configuration/dhcp6/eth1",
		},
	)
}

func (suite *OperatorConfigSuite) TestMachineConfigurationWithAliases() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.OperatorConfigController{},
		),
	)
	// add LinkConfig controller to produce link specs based on machine configuration
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.LinkConfigController{},
		),
	)

	for _, link := range []struct {
		name    string
		aliases []string
	}{
		{
			name:    "eth0",
			aliases: []string{"enx0123"},
		},
		{
			name:    "eth1",
			aliases: []string{"enx0456"},
		},
		{
			name:    "eth2",
			aliases: []string{"enxa"},
		},
		{
			name:    "eth3",
			aliases: []string{"enxb"},
		},
		{
			name:    "eth4",
			aliases: []string{"enxc"},
		},
	} {
		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().AltNames = link.aliases
		status.TypedSpec().Type = nethelpers.LinkEther
		status.TypedSpec().LinkState = true

		suite.Create(status)
	}

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
								DeviceInterface: "enx0123",
							},
							{
								DeviceInterface: "enx0456",
								DeviceDHCP:      pointer.To(true),
							},
							{
								DeviceIgnore:    pointer.To(true),
								DeviceInterface: "enxa",
								DeviceDHCP:      pointer.To(true),
							},
							{
								DeviceInterface: "enxb",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv4:        pointer.To(true),
									DHCPRouteMetric: 256,
								},
							},
							{
								DeviceInterface: "enxc",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID:   25,
										VlanDHCP: pointer.To(true),
									},
									{
										VlanID: 26,
									},
									{
										VlanID: 27,
										VlanDHCPOptions: &v1alpha1.DHCPOptions{
											DHCPRouteMetric: 256,
										},
									},
								},
							},
							{
								DeviceInterface: "enxd",
								DeviceDHCP:      pointer.To(true),
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

	suite.assertOperators(
		[]string{
			"configuration/dhcp4/eth1",
			"configuration/dhcp4/eth3",
			"configuration/dhcp4/enxc.25",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/dhcp4/eth1":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth3":
				asrt.Equal("eth3", r.TypedSpec().LinkName)
				asrt.EqualValues(256, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/enxc.25":
				asrt.Equal("enxc.25", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			}
		},
	)

	suite.assertNoOperators(
		[]string{
			"configuration/dhcp4/eth0",
			"default/dhcp4/eth0",
			"configuration/dhcp4/eth2",
			"default/dhcp4/eth2",
			"configuration/dhcp4/eth4.26",
		},
	)
}

func TestOperatorConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &OperatorConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.DeviceConfigController{}))
			},
		},
	})
}
