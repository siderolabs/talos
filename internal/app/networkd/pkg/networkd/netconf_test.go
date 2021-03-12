// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package networkd

import (
	"log"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
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
		_, opts, err := buildOptions(log.New(os.Stderr, "", log.LstdFlags), device, "")
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

func sampleConfig() []config.Device {
	return []config.Device{
		&v1alpha1.Device{
			DeviceInterface: "eth0",
			DeviceCIDR:      "192.168.0.10/24",
		},
		&v1alpha1.Device{
			DeviceInterface: "bond0",
			DeviceCIDR:      "192.168.0.10/24",
			DeviceBond:      &v1alpha1.Bond{BondInterfaces: []string{"lo"}},
		},
		&v1alpha1.Device{
			DeviceInterface: "bond0",
			DeviceBond:      &v1alpha1.Bond{BondInterfaces: []string{"lo"}, BondMode: "balance-rr"},
		},
		&v1alpha1.Device{
			DeviceInterface: "eth0",
			DeviceIgnore:    true,
		},
		&v1alpha1.Device{
			DeviceInterface: "eth0",
			DeviceMTU:       9100,
			DeviceCIDR:      "192.168.0.10/24",
			DeviceRoutes:    []*v1alpha1.Route{{RouteNetwork: "10.0.0.0/8", RouteGateway: "10.0.0.1"}},
		},
		&v1alpha1.Device{
			DeviceInterface: "bond0",
			DeviceBond: &v1alpha1.Bond{
				BondInterfaces: []string{"lo"},
				BondMode:       "balance-rr",
				BondHashPolicy: "layer2",
				BondLACPRate:   "fast",
				BondMIIMon:     200,
				BondUpDelay:    100,
				BondDownDelay:  100,
			},
		},
		&v1alpha1.Device{
			DeviceInterface: "bondyolo0",
			DeviceBond: &v1alpha1.Bond{
				BondInterfaces:      []string{"lo"},
				BondMode:            "balance-rr",
				BondHashPolicy:      "layer2",
				BondLACPRate:        "fast",
				BondMIIMon:          200,
				BondUpDelay:         100,
				BondDownDelay:       100,
				BondUseCarrier:      false,
				BondARPInterval:     230,
				BondARPValidate:     "all",
				BondARPAllTargets:   "all",
				BondPrimary:         "lo",
				BondPrimaryReselect: "better",
				BondFailOverMac:     "none",
				BondResendIGMP:      10,
				BondNumPeerNotif:    5,
				BondAllSlavesActive: 1,
				BondMinLinks:        1,
				BondLPInterval:      100,
				BondPacketsPerSlave: 50,
				BondADSelect:        "bandwidth",
				BondADActorSysPrio:  23,
				BondADUserPortKey:   323,
				BondTLBDynamicLB:    1,
				BondPeerNotifyDelay: 200,
			},
		},
	}
}

func sampleKernelIPParam() string {
	return "1.1.1.1:2.2.2.2:3.3.3.3:255.255.255.0:hostname:eth0:none:4.4.4.4:5.5.5.5:6.6.6.6"
}
