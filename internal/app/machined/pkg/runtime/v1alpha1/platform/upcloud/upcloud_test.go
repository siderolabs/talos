// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package upcloud_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/upcloud"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkConfig() {
	cfg := []byte(`{
"cloud_name": "upcloud",
"instance_id": "00123456-1111-2222-3333-123456789012",
"hostname": "talos",
"network": {
	"interfaces": [
	{
		"index": 1,
		"ip_addresses": [
			{
				"address": "185.70.197.2",
				"dhcp": true,
				"dns": [
				"94.237.127.9",
				"94.237.40.9"
				],
				"family": "IPv4",
				"floating": false,
				"gateway": "185.70.196.1",
				"network": "185.70.196.0/22"
			},
			{
				"address": "185.70.197.3",
				"dhcp": false,
				"dns": null,
				"family": "IPv4",
				"floating": true,
				"gateway": "",
				"network": "185.70.197.3/32"
			}
		],
		"mac": "5e:bf:5e:02:28:07",
		"network_id": "035ef879-1111-2222-3333-123456789012",
		"type": "public"
	},
	{
		"index": 2,
		"ip_addresses": [
		{
			"address": "10.11.0.2",
			"dhcp": true,
			"dns": null,
			"family": "IPv4",
			"floating": false,
			"gateway": "10.11.0.1",
			"network": "10.11.0.0/22"
		}
		],
		"mac": "5e:bf:5e:02:cd:e0",
		"network_id": "031c9f9c-1111-2222-3333-123456789012",
		"type": "utility"
	},
	{
		"index": 3,
		"ip_addresses": [
		{
			"address": "2a04:3544:8000:1000:0000:1111:2222:3333",
			"dhcp": true,
			"dns": [
			"2a04:3540:53::1",
			"2a04:3544:53::1"
			],
			"family": "IPv6",
			"floating": false,
			"gateway": "2a04:3544:8000:1000::1",
			"network": "2a04:3544:8000:1000::/64"
		}
		],
		"mac": "5e:bf:5e:02:78:a4",
		"network_id": "03b326a2-1111-2222-3333-123456789012",
		"type": "public"
	}
	],
	"dns": [
		"94.237.127.9",
		"94.237.40.9"
	]
},
"storage": {},
"tags": [],
"user_data": "",
"vendor_data": ""
}`)

	p := &upcloud.UpCloud{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceAddresses: []string{"185.70.197.3/32"},
						DeviceDHCP:      true,
					},
					{
						DeviceInterface: "eth1",
						DeviceDHCP:      true,
					},
					{
						DeviceInterface: "eth2",
						DeviceDHCP:      false,
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
