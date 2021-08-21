// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway_test

import (
	"encoding/json"
	"testing"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/scaleway"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkConfig() {
	cfg := []byte(`{
"id": "11111111-1111-1111-1111-111111111111",
"name": "scw-talos",
"commercial_type": "DEV1-S",
"hostname": "scw-talos",
"tags": [],
"state_detail": "booted",
"public_ip": {
	"id": "11111111-1111-1111-1111-111111111111",
	"address": "11.22.222.222",
	"dynamic": false
},
"private_ip": "10.00.222.222",
"ipv6": {
	"address": "2001:111:222:3333::1",
	"gateway": "2001:111:222:3333::",
	"netmask": "64"
}
}`)

	metadata := &instance.Metadata{}
	err := json.Unmarshal(cfg, &metadata)
	suite.Require().NoError(err)

	p := &scaleway.Scaleway{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceDHCP:      true,
						DeviceAddresses: []string{"2001:111:222:3333::1/64"},
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteNetwork: "::/0",
								RouteGateway: "2001:111:222:3333::",
								RouteMetric:  1024,
							},
						},
					},
				},
			},
		},
	}

	result, err := p.ConfigurationNetwork(metadata, defaultMachineConfig)

	suite.Require().NoError(err)
	suite.Assert().Equal(machineConfig, result)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
