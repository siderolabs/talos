// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	configs "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type DeviceConfigSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *DeviceConfigSpecSuite) TestDeviceConfigs() {
	cfgProvider := container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceAddresses: []string{"192.168.2.0/24"},
						DeviceMTU:       1500,
					},
					{
						DeviceInterface: "bond0",
						DeviceAddresses: []string{"192.168.2.0/24"},
						DeviceBond: &v1alpha1.Bond{
							BondMode:       "balance-rr",
							BondInterfaces: []string{"eth1", "eth2"},
						},
					},
					{
						DeviceInterface: "eth0",
						DeviceAddresses: []string{"192.168.3.0/24"},
					},
				},
			},
		},
	})

	cfg := config.NewMachineConfig(cfgProvider)

	devices := map[string]configs.Device{}
	for index, item := range cfgProvider.Machine().Network().Devices() {
		devices[fmt.Sprintf("%s/%03d", item.Interface(), index)] = item
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), maps.Keys(devices),
		func(r *network.DeviceConfigSpec, assert *assert.Assertions) {
			assert.Equal(r.TypedSpec().Device, devices[r.Metadata().ID()])
		},
	)
}

func (suite *DeviceConfigSpecSuite) TestSelectors() {
	kernelDriver := "thedriver"

	cfgProvider := container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					// device selector selecing a single interface
					{
						DeviceSelector: &v1alpha1.NetworkDeviceSelector{
							NetworkDeviceKernelDriver: kernelDriver,
						},
						DeviceAddresses: []string{"192.168.2.0/24"},
						DeviceMTU:       1500,
					},
					// no device selector (explicit name)
					{
						DeviceInterface: "eth0",
						DeviceAddresses: []string{"192.168.3.0/24"},
					},
					// device selector which doesn't match anything
					{
						DeviceSelector: &v1alpha1.NetworkDeviceSelector{
							NetworkDeviceKernelDriver: "no-match",
						},
						DeviceAddresses: []string{"192.168.4.0/24"},
					},
					// device selector which matches multiple interfaces
					{
						DeviceSelector: &v1alpha1.NetworkDeviceSelector{
							NetworkDeviceBus: "0000:01*",
						},
						DeviceAddresses: []string{"192.168.5.0/24"},
					},
					// device selector which matches physical interfaces
					{
						DeviceSelector: &v1alpha1.NetworkDeviceSelector{
							NetworkDevicePhysical: pointer.To(true),
						},
						DeviceAddresses: []string{"192.168.6.0/24"},
					},
				},
			},
		},
	})

	cfg := config.NewMachineConfig(cfgProvider)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	status := network.NewLinkStatus(network.NamespaceName, "eth0")
	status.TypedSpec().Driver = kernelDriver
	status.TypedSpec().BusPath = "0000:01:00.0"
	status.TypedSpec().Type = nethelpers.LinkEther // physical
	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))

	status = network.NewLinkStatus(network.NamespaceName, "eth1")
	status.TypedSpec().BusPath = "0000:01:01.0"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"eth0/000"},
		func(r *network.DeviceConfigSpec, assert *assert.Assertions) {
			assert.Equal(1500, r.TypedSpec().Device.MTU())
			assert.Equal([]string{"192.168.2.0/24"}, r.TypedSpec().Device.Addresses())
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"eth0/001"},
		func(r *network.DeviceConfigSpec, assert *assert.Assertions) {
			assert.Equal([]string{"192.168.3.0/24"}, r.TypedSpec().Device.Addresses())
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"eth0/003/000", "eth1/003/001"},
		func(r *network.DeviceConfigSpec, assert *assert.Assertions) {
			assert.Equal([]string{"192.168.5.0/24"}, r.TypedSpec().Device.Addresses())
		},
	)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"eth0/004"},
		func(r *network.DeviceConfigSpec, assert *assert.Assertions) {
			assert.Equal([]string{"192.168.6.0/24"}, r.TypedSpec().Device.Addresses())
		},
	)
}

func (suite *DeviceConfigSpecSuite) TestBondSelectors() {
	cfgProvider := container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "bond0",
						DeviceAddresses: []string{"192.168.2.0/24"},
						DeviceMTU:       1500,
						DeviceBond: &v1alpha1.Bond{
							BondMode: "balance-rr",
							BondDeviceSelectors: []v1alpha1.NetworkDeviceSelector{
								{
									NetworkDevicePermanentAddress: "00:*",
								},
								{
									NetworkDevicePermanentAddress: "01:*",
								},
							},
						},
					},
				},
			},
		},
	})

	cfg := config.NewMachineConfig(cfgProvider)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	for _, link := range []string{"eth0", "eth1"} {
		status := network.NewLinkStatus(network.NamespaceName, link)
		suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
	}

	rtestutils.AssertNoResource[*network.DeviceConfigSpec](suite.Ctx(), suite.T(), suite.State(), "bond0/000")

	for _, link := range []struct {
		name   string
		hwaddr string
	}{
		{
			name:   "bond0",
			hwaddr: "00:11:22:33:44:55", // bond0 will inherit MAC of the first link
		},
		{
			name:   "eth3",
			hwaddr: "00:11:22:33:44:55",
		},
		{
			name:   "eth4",
			hwaddr: "01:11:22:33:44:55",
		},
		{
			name:   "eth5",
			hwaddr: "01:11:22:33:44:ef",
		},
		{
			name:   "eth6",
			hwaddr: "02:11:22:33:44:55",
		},
	} {
		hwaddr, err := net.ParseMAC(link.hwaddr)
		suite.Require().NoError(err)

		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().PermanentAddr = nethelpers.HardwareAddr(hwaddr)
		suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
	}

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"bond0/000"},
		func(r *network.DeviceConfigSpec, assert *assert.Assertions) {
			assert.Equal(1500, r.TypedSpec().Device.MTU())
			assert.Equal([]string{"192.168.2.0/24"}, r.TypedSpec().Device.Addresses())
			assert.Equal([]string{"eth3", "eth4", "eth5"}, r.TypedSpec().Device.Bond().Interfaces())
		},
	)
}

func TestDeviceConfigSpecSuite(t *testing.T) {
	suite.Run(t, &DeviceConfigSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DeviceConfigController{}))
			},
		},
	})
}
