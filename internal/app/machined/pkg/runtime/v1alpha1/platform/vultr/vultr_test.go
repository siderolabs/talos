// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vultr_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vultr"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestNetworkConfig() {
	//nolint:lll
	cfg := []byte(`{
"bgp":{"ipv4":{"my-address":"","my-asn":"","peer-address":"","peer-asn":""},"ipv6":{"my-address":"","my-asn":"","peer-address":"","peer-asn":""}},"hostname":"talos","instance-v2-id":"91b07056-af72-4551-b15b-d57d34071be9","instanceid":"50190000","interfaces":[{"ipv4":{"additional":[],"address":"95.111.222.111","gateway":"95.111.222.1","netmask":"255.255.254.0"},"ipv6":{"additional":[],"address":"2001:19f0:5001:2095:1111:2222:3333:4444","network":"2001:19f0:5001:2095::","prefix":"64"},"mac":"56:00:03:89:53:e0","network-type":"public"},{"ipv4":{"additional":[],"address":"10.7.96.3","gateway":"","netmask":"255.255.240.0"},"ipv6":{"additional":[],"network":"","prefix":""},"mac":"5a:00:03:89:53:e0","network-type":"private","network-v2-id":"dadc2b30-0b55-4fa1-8c29-f67215bd5ac4","networkid":"net6126811851cd7"}],"public-keys":["ssh-ed25519"],"region":{"regioncode":"AMS"},"user-defined":[]
}`)

	p := &vultr.Vultr{}

	defaultMachineConfig := &v1alpha1.Config{}

	machineConfig := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceDHCP:      true,
					},
					{
						DeviceInterface: "eth1",
						DeviceAddresses: []string{"10.7.96.3/20"},
						DeviceDHCP:      false,
						DeviceMTU:       1450,
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
