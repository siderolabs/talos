// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"net"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
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
	conf := sampleConfig()
	eth0 := &net.Interface{Index: 1, MTU: 1500, Name: "eth0"}
	nc := NetConf{eth0: []nic.Option{nic.WithName(eth0.Name)}}
	err := nc.BuildOptions(conf)
	suite.Assert().NoError(err)

	iface, err := nic.Create(eth0, nc[eth0]...)
	suite.Assert().NoError(err)

	suite.Assert().Equal(iface.AddressMethod[0].Resolvers()[0], net.ParseIP(conf.Machine().Network().Resolvers()[0]))
	suite.Assert().Equal(iface.AddressMethod[0].Resolvers()[1], net.ParseIP(conf.Machine().Network().Resolvers()[1]))
	suite.Assert().Equal(int(iface.AddressMethod[0].MTU()), conf.Machine().Network().Devices()[0].MTU)
	// nolint: errcheck
	addr, _, _ := net.ParseCIDR(conf.Machine().Network().Devices()[0].CIDR)
	suite.Assert().Equal(iface.AddressMethod[0].Address().IP, addr)
}

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
