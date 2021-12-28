// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestOperatorSpecMarshalYAML(t *testing.T) {
	spec := network.OperatorSpecSpec{
		Operator:  network.OperatorDHCP4,
		LinkName:  "eth0",
		RequireUp: true,

		DHCP4: network.DHCP4OperatorSpec{
			RouteMetric: 1024,
		},
		DHCP6: network.DHCP6OperatorSpec{
			RouteMetric: 1024,
		},
		VIP: network.VIPOperatorSpec{
			IP:            netaddr.MustParseIP("192.168.1.1"),
			GratuitousARP: true,
			EquinixMetal: network.VIPEquinixMetalSpec{
				ProjectID: "a",
				DeviceID:  "b",
				APIToken:  "c",
			},
			HCloud: network.VIPHCloudSpec{
				DeviceID:  3,
				NetworkID: 4,
				APIToken:  "d",
			},
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t,
		`operator: dhcp4
linkName: eth0
requireUp: true
dhcp4:
    routeMetric: 1024
dhcp6:
    routeMetric: 1024
vip:
    ip: 192.168.1.1
    gratuitousARP: true
    equinixMetal:
        projectID: a
        deviceID: b
        apiToken: c
    hcloud:
        deviceID: 3
        networkID: 4
        apiToken: d
layer: configuration
`,
		string(marshaled))

	var spec2 network.OperatorSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}
