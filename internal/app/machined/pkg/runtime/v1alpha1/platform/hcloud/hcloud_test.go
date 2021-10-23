// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/hcloud"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkConfig() {
	cfg := []byte(`
config:
- mac_address: 96:00:00:1:2:3
  name: eth0
  subnets:
  - ipv4: true
    type: dhcp
  - address: 2a01:4f8:1:2::1/64
    gateway: fe80::1
    ipv6: true
    type: static
  type: physical
- address:
  - 185.12.64.2
  - 185.12.64.1
  interface: eth0
  type: nameserver
version: 1
`)
	p := &hcloud.Hcloud{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceDHCP:      true,
						DeviceAddresses: []string{"2a01:4f8:1:2::1/64"},
						DeviceRoutes: []*v1alpha1.Route{
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

	result, err := p.ConfigurationNetwork(cfg, defaultMachineConfig)

	suite.Require().NoError(err)
	suite.Assert().Equal(machineConfig, result)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
