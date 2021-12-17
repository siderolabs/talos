// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package oracle_test

import (
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/oracle"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkConfig() {
	cfg := []byte(`
[ {
  "vnicId" : "ocid1.vnic.oc1.eu-amsterdam-1.asdasd",
  "privateIp" : "172.16.1.11",
  "vlanTag" : 1,
  "macAddr" : "02:00:17:00:00:00",
  "virtualRouterIp" : "172.16.1.1",
  "subnetCidrBlock" : "172.16.1.0/24",
  "ipv6SubnetCidrBlock" : "2603:a:b:c::/64",
  "ipv6VirtualRouterIp" : "fe80::a:b:c:d"
} ]
`)
	a := &oracle.Oracle{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface:   "eth0",
						DeviceDHCP:        true,
						DeviceDHCPOptions: &v1alpha1.DHCPOptions{DHCPIPv6: pointer.ToBool(true)},
					},
				},
			},
		},
	}

	result, err := a.ConfigurationNetwork(cfg, defaultMachineConfig)

	suite.Require().NoError(err)
	suite.Assert().Equal(machineConfig, result)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
