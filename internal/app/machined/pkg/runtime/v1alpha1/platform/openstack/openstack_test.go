// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/openstack"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

// https://specs.openstack.org/openstack/nova-specs/specs/liberty/implemented/metadata-service-network-info.html

func (suite *ConfigSuite) TestNetworkConfig() {
	cfg := []byte(`{
"links": [
   {
      "ethernet_mac_address": "A4:BF:00:10:20:30",
      "id": "aae16046-6c74-4f33-acf2-a16e9ab093eb",
      "type": "phy",
      "mtu": 1450,
      "vif_id": "7607af2d-c24d-4bfb-909e-c447b119f4e2"
   },
   {
       "ethernet_mac_address": "A4:BF:00:10:20:31",
       "id": "aae16046-6c74-4f33-acf2-a16e9ab093ec",
       "type": "ovs",
       "mtu": 9000,
       "vif_id": "c816df7e-7bcc-45ca-9eb2-3d3d3dca0639"
   }
],
"networks": [
   {
      "id": "publicnet-ipv4",
      "link": "aae16046-6c74-4f33-acf2-a16e9ab093eb",
      "network_id": "66374c4d-5123-4f11-8fa9-8a6dea2b4fe7",
      "type": "ipv4_dhcp"
   },
   {
      "routes": [
         {
            "network": "2000:0:100:2f00::",
            "gateway": "2000:0:100:2fff:ff:ff:ff:f0",
            "netmask": "ffff:ffff:ffff:ffc0::"
         }
      ],
      "dns_nameservers": [
            "2000:0:100::1"
      ],
      "gateway": "2000:0:100:2fff:ff:ff:ff:ff",
      "link": "aae16046-6c74-4f33-acf2-a16e9ab093eb",
      "ip_address": "2000:0:100::/56",
      "network_id": "39b48637-d98a-4dfc-a05b-d61e8d88fafe",
      "id": "publicnet-ipv6",
      "type": "ipv6"
   },
   {
      "id": "privatnet-ipv4",
      "link": "aae16046-6c74-4f33-acf2-a16e9ab093ec",
      "network_id": "66374c4d-5123-4f11-8fa9-8a6dea2b4fe7",
      "type": "ipv4_dhcp"
   },
   {
      "routes": [
         {
            "network": "::",
            "netmask": "::",
            "gateway": "2000:0:ff00::"
          }
      ],
      "id": "privatnet-ipv6",
      "link": "aae16046-6c74-4f33-acf2-a16e9ab093ec",
      "ip_address": "2000:0:ff00::1",
      "netmask": "ffff:ffff:ffff:ff00::",
      "network_id": "66374c4d-5123-4f11-8fa9-8a6dea2b4fe7",
      "type": "ipv6"
   },
],
"services": [
   {
      "address": "8.8.8.8",
      "type": "dns"
   },
   {
      "address": "1.1.1.1",
      "type": "dns"
   }
]
}`)

	meta := []byte(`{
"availability_zone": "nova",
"devices": [],
"hostname": "talos",
"keys": [],
"launch_index": 0,
"name": "talos",
"project_id": "39073b0a-1234-1234-1234-5e76a4bd64b2",
"public_keys": {},
"uuid": "39073b0a-1234-1234-1234-5e76a4bd64b2"
}`)

	p := &openstack.Openstack{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkHostname: "talos",
				NameServers:     []string{"8.8.8.8", "1.1.1.1"},
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceMTU:       1450,
						DeviceDHCP:      true,
						DeviceAddresses: []string{"2000:0:100::/56"},
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteNetwork: "::/0",
								RouteGateway: "2000:0:100:2fff:ff:ff:ff:ff",
								RouteMetric:  1024,
							},
							{
								RouteNetwork: "2000:0:100:2f00::/58",
								RouteGateway: "2000:0:100:2fff:ff:ff:ff:f0",
								RouteMetric:  1024,
							},
						},
					},
					{
						DeviceInterface: "eth1",
						DeviceMTU:       9000,
						DeviceDHCP:      true,
						DeviceAddresses: []string{"2000:0:ff00::1/56"},
						DeviceRoutes: []*v1alpha1.Route{
							{
								RouteNetwork: "::/0",
								RouteGateway: "2000:0:ff00::",
								RouteMetric:  1024,
							},
						},
					},
				},
			},
		},
	}

	result, err := p.ConfigurationNetwork(cfg, meta, defaultMachineConfig)

	suite.Require().NoError(err)
	suite.Assert().Equal(machineConfig, result)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
