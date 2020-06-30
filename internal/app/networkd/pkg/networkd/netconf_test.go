// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint: testpackage
package networkd

import (
	"net"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
)

type NetconfSuite struct {
	suite.Suite
}

func TestNetconfSuite(t *testing.T) {
	// Hide all our state transition messages
	// log.SetOutput(ioutil.Discard)
	suite.Run(t, new(NetconfSuite))
}

func (suite *NetconfSuite) TestBaseNetconf() {
	for _, device := range sampleConfig() {
		_, opts, err := buildOptions(device, "")
		suite.Require().NoError(err)

		_, err = nic.New(opts...)
		suite.Require().NoError(err)
	}
}

func (suite *NetconfSuite) TestKernelNetconf() {
	name, opts := buildKernelOptions(sampleKernelIPParam())

	iface, err := nic.New(opts...)
	suite.Require().NoError(err)

	suite.Assert().Equal(iface.Name, name)
	suite.Assert().Equal(len(iface.AddressMethod), 1)
	addr := iface.AddressMethod[0]
	suite.Assert().Equal(addr.Name(), "static")
	suite.Assert().Equal(addr.Hostname(), "hostname")
	suite.Assert().Equal(addr.Address().IP, net.ParseIP("1.1.1.1"))
	suite.Assert().Equal(len(addr.Resolvers()), 2)
	suite.Assert().Equal(addr.Resolvers()[0], net.ParseIP("4.4.4.4"))
	suite.Assert().Equal(addr.Resolvers()[1], net.ParseIP("5.5.5.5"))
	suite.Assert().Equal(len(addr.Routes()), 1)
}

func (suite *NetconfSuite) TestKernelNetconfIncomplete() {
	name, opts := buildKernelOptions("1.1.1.1::3.3.3.3:255.255.255.0::eth0:none:::")

	iface, err := nic.New(opts...)
	suite.Require().NoError(err)

	suite.Assert().Equal(iface.Name, name)
	suite.Assert().Equal(len(iface.AddressMethod), 1)
	addr := iface.AddressMethod[0]
	suite.Assert().Equal(addr.Name(), "static")
	suite.Assert().Equal(addr.Hostname(), "")
	suite.Assert().Equal(addr.Address().IP, net.ParseIP("1.1.1.1"))
	suite.Assert().Len(addr.Resolvers(), 0)
	suite.Assert().Equal(len(addr.Routes()), 1)
}

func sampleConfig() []runtime.Device {
	return []runtime.Device{
		{
			Interface: "eth0",
			CIDR:      "192.168.0.10/24",
		},
		{
			Interface: "bond0",
			CIDR:      "192.168.0.10/24",
			Bond:      &runtime.Bond{Interfaces: []string{"lo"}},
		},
		{
			Interface: "bond0",
			Bond:      &runtime.Bond{Interfaces: []string{"lo"}, Mode: "balance-rr"},
		},
		{
			Interface: "eth0",
			Ignore:    true,
		},
		{
			Interface: "eth0",
			MTU:       9100,
			CIDR:      "192.168.0.10/24",
			Routes:    []runtime.Route{{Network: "10.0.0.0/8", Gateway: "10.0.0.1"}},
		},
		{
			Interface: "bond0",
			Bond: &runtime.Bond{
				Interfaces: []string{"lo"},
				Mode:       "balance-rr",
				HashPolicy: "layer2",
				LACPRate:   "fast",
				MIIMon:     200,
				UpDelay:    100,
				DownDelay:  100,
			},
		},
		{
			Interface: "bondyolo0",
			Bond: &runtime.Bond{
				Interfaces:      []string{"lo"},
				Mode:            "balance-rr",
				HashPolicy:      "layer2",
				LACPRate:        "fast",
				MIIMon:          200,
				UpDelay:         100,
				DownDelay:       100,
				UseCarrier:      false,
				ARPInterval:     230,
				ARPValidate:     "all",
				ARPAllTargets:   "all",
				Primary:         "lo",
				PrimaryReselect: "better",
				FailOverMac:     "none",
				ResendIGMP:      10,
				NumPeerNotif:    5,
				AllSlavesActive: 1,
				MinLinks:        1,
				LPInterval:      100,
				PacketsPerSlave: 50,
				ADSelect:        "bandwidth",
				ADActorSysPrio:  23,
				ADUserPortKey:   323,
				TLBDynamicLB:    1,
				PeerNotifyDelay: 200,
			},
		},
	}
}

func sampleKernelIPParam() string {
	return "1.1.1.1:2.2.2.2:3.3.3.3:255.255.255.0:hostname:eth0:none:4.4.4.4:5.5.5.5:6.6.6.6"
}
