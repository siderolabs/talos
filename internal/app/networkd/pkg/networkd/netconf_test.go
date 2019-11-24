// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/config/machine"
)

type NetconfSuite struct {
	suite.Suite
}

func TestNetconfSuite(t *testing.T) {
	// Hide all our state transition messages
	// log.SetOutput(ioutil.Discard)
	suite.Run(t, new(NetconfSuite))
}

func (suite *NetconfSuite) TestNetconf() {
	for _, device := range sampleConfig() {
		_, opts, err := buildOptions(device)
		suite.Require().NoError(err)

		_, err = nic.New(opts...)
		suite.Require().NoError(err)
	}
}

func sampleConfig() []machine.Device {
	return []machine.Device{
		{
			Interface: "eth0",
			CIDR:      "192.168.0.10/24",
		},
		{
			Interface: "bond0",
			CIDR:      "192.168.0.10/24",
			Bond:      &machine.Bond{Interfaces: []string{"lo"}},
		},
		{
			Interface: "bond0",
			Bond:      &machine.Bond{Interfaces: []string{"lo"}, Mode: "balance-rr"},
		},
		{
			Interface: "eth0",
			Ignore:    true,
		},
		{
			Interface: "eth0",
			MTU:       9100,
			CIDR:      "192.168.0.10/24",
			Routes:    []machine.Route{{Network: "10.0.0.0/8", Gateway: "10.0.0.1"}},
		},
		{
			Interface: "bond0",
			Bond: &machine.Bond{
				Interfaces: []string{"lo"},
				Mode:       "balance-rr",
				HashPolicy: "layer2",
				LACPRate:   "fast",
				MIIMon:     200,
				UpDelay:    100,
				DownDelay:  100,
			},
		},
	}
}

/*
func sampleConfig() runtime.Configurator {
	return &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NameServers:     []string{"1.2.3.4", "2.3.4.5"},
				NetworkHostname: "myhostname",
				NetworkInterfaces: []machine.Device{
					{
						Interface: "eth0",
						CIDR:      "192.168.0.10/24",
						MTU:       9100,
						DHCP:      false,
					},
				},
			},
		},
	}
}
*/
