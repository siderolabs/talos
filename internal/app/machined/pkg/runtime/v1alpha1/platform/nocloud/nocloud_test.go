// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/nocloud"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkConfigV1() {
	cfg := []byte(`
version: 1
config:
  - type: physical
    name: eth0
    mac_address: 'ae:71:9e:61:d0:ad'
    subnets:
    - type: static
      address: '192.168.1.11'
      netmask: '255.255.255.0'
      gateway: '192.168.1.1'
    - type: static6
      address: '2001:2:3:4:5:6:7:8/64'
      gateway: 'fe80::1'
  - type: nameserver
    address:
    - '192.168.1.1'
    search:
    - 'lan'
`)
	p := &nocloud.Nocloud{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkHostname: "talos",
				NameServers:     []string{"192.168.1.1"},
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceDHCP:      false,
						DeviceAddresses: []string{"192.168.1.11/24", "2001:2:3:4:5:6:7:8/64"},
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteNetwork: "0.0.0.0/0",
								RouteGateway: "192.168.1.1",
								RouteMetric:  1024,
							},
							{
								RouteNetwork: "::/0",
								RouteGateway: "fe80::1",
								RouteMetric:  1024,
							},
						},
					},
				},
			},
		},
	}

	result, err := p.ConfigurationNetwork(cfg, []byte("hostname: talos"), defaultMachineConfig)

	suite.Require().NoError(err)
	suite.Assert().Equal(machineConfig, result)
}

// Network configs-v2 examples https://github.com/canonical/netplan/tree/main/examples

func (suite *ConfigSuite) TestNetworkConfigV2() {
	cfg := []byte(`
version: 2
ethernets:
  eth0:
    dhcp4: true
    addresses:
      - 192.168.14.2/24
      - 2001:1::1/64
    gateway4: 192.168.14.1
    gateway6: 2001:1::2
    nameservers:
      search: [foo.local, bar.local]
      addresses: [8.8.8.8]
`)
	p := &nocloud.Nocloud{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NameServers: []string{"8.8.8.8"},
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceDHCP:      true,
						DeviceAddresses: []string{"192.168.14.2/24", "2001:1::1/64"},
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteNetwork: "0.0.0.0/0",
								RouteGateway: "192.168.14.1",
								RouteMetric:  1024,
							},
							{
								RouteNetwork: "::/0",
								RouteGateway: "2001:1::2",
								RouteMetric:  1024,
							},
						},
					},
				},
			},
		},
	}

	result, err := p.ConfigurationNetwork(cfg, []byte{}, defaultMachineConfig)

	suite.Require().NoError(err)
	suite.Assert().Equal(machineConfig, result)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
