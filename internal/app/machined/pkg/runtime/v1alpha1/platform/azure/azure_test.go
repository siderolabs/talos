// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure_test

import (
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/azure"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkConfig() {
	cfg := []byte(`
[
  {
    "ipv4": {
      "ipAddress": [
        {
          "privateIpAddress": "172.18.1.10",
          "publicIpAddress": "1.2.3.4"
        }
      ],
      "subnet": [
        {
          "address": "172.18.1.0",
          "prefix": "24"
        }
      ]
    },
    "ipv6": {
      "ipAddress": [
        {
            "privateIpAddress": "fd00::10",
            "publicIpAddress": ""
        }
       ]
    },
    "macAddress": "000D3AD631EE"
  }
]
`)
	a := &azure.Azure{}

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
