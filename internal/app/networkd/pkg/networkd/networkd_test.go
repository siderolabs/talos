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

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	// Hide all our state transition messages
	// log.SetOutput(ioutil.Discard)
	suite.Run(t, new(NetworkdSuite))
}

func (suite *NetworkdSuite) TestNetworkd() {
	nwd, err := New(log.New(os.Stderr, "", log.LstdFlags), sampleConfigFile())
	suite.Require().NoError(err)

	suite.Require().Contains(nwd.Interfaces, "eth0")
	suite.Assert().False(nwd.Interfaces["eth0"].Bonded)
	suite.Require().Contains(nwd.Interfaces, "bond0")
	suite.Assert().True(nwd.Interfaces["bond0"].Bonded)
	suite.Assert().Equal(1, len(nwd.Interfaces["bond0"].SubInterfaces))
	suite.Require().Contains(nwd.Interfaces, "lo")
}

func (suite *NetworkdSuite) TestHostname() {
	var (
		addr         net.IP
		domainname   string
		err          error
		hostname     string
		nwd          *Networkd
		sampleConfig config.Provider
	)

	nwd, err = New(log.New(os.Stderr, "", log.LstdFlags), nil)
	suite.Require().NoError(err)

	// Default test
	hostname, _, addr, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("talos-127-0-1-1", hostname)
	suite.Assert().Equal(addr, net.ParseIP("127.0.1.1"))

	// Static addressing tests

	// Static with hostname
	sampleConfig = sampleConfigFile()

	nwd, err = New(log.New(os.Stderr, "", log.LstdFlags), sampleConfig)
	suite.Require().NoError(err)

	hostname, _, addr, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("myhostname", hostname)
	suite.Assert().Equal(addr, net.ParseIP("192.168.0.10"))

	// Static for computed hostname ( talos-ip )
	sampleConfig.(*v1alpha1.Config).MachineConfig.MachineNetwork.NetworkHostname = ""

	nwd, err = New(log.New(os.Stderr, "", log.LstdFlags), sampleConfig)
	suite.Require().NoError(err)

	hostname, _, addr, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("talos-192-168-0-10", hostname)
	suite.Assert().Equal(addr, net.ParseIP("192.168.0.10"))

	// Static for hostname too long
	sampleConfig.(*v1alpha1.Config).MachineConfig.MachineNetwork.NetworkHostname = "somereallyreallyreallylongstringthathasmorethan63charactersbecauseweneedtotestit"

	nwd, err = New(log.New(os.Stderr, "", log.LstdFlags), sampleConfig)
	suite.Require().NoError(err)

	//nolint:dogsled
	_, _, _, err = nwd.decideHostname()
	suite.Require().Error(err)

	// Static for hostname vs domain name
	sampleConfig.(*v1alpha1.Config).MachineConfig.MachineNetwork.NetworkHostname = "dadjokes.biz.dev.com.org.io"

	nwd, err = New(log.New(os.Stderr, "", log.LstdFlags), sampleConfig)
	suite.Require().NoError(err)

	hostname, domainname, _, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("dadjokes", hostname)
	suite.Assert().Equal("biz.dev.com.org.io", domainname)

	// DHCP addressing tests

	// DHCP with OptionHostName
	nwd, err = New(log.New(os.Stderr, "", log.LstdFlags), dhcpConfigFile())
	suite.Require().NoError(err)

	nwd.Interfaces["eth0"].AddressMethod = []address.Addressing{
		&address.DHCP4{
			Ack: &dhcpv4.DHCPv4{
				YourIPAddr: net.ParseIP("192.168.0.11"),
				Options: dhcpv4.Options{
					uint8(dhcpv4.OptionHostName):   []byte("evenbetterdadjokes"),
					uint8(dhcpv4.OptionSubnetMask): []byte{255, 255, 255, 0},
				},
			},
		},
	}

	hostname, _, addr, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("evenbetterdadjokes", hostname)
	suite.Assert().Equal(addr.String(), "192.168.0.11")

	// DHCP without OptionHostName
	nwd, err = New(log.New(os.Stderr, "", log.LstdFlags), dhcpConfigFile())
	suite.Require().NoError(err)

	nwd.Interfaces["eth0"].AddressMethod = []address.Addressing{
		&address.DHCP4{
			Ack: &dhcpv4.DHCPv4{
				YourIPAddr: net.ParseIP("192.168.0.11"),
				Options: dhcpv4.Options{
					uint8(dhcpv4.OptionSubnetMask): []byte{255, 255, 255, 0},
				},
			},
		},
	}

	hostname, _, addr, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("talos-192-168-0-11", hostname)
	suite.Assert().Equal(addr, net.ParseIP("192.168.0.11"))

	// DHCP without OptionHostname and with OptionDomainName
	nwd.Interfaces["eth0"].AddressMethod = []address.Addressing{
		&address.DHCP4{
			Ack: &dhcpv4.DHCPv4{
				YourIPAddr: net.ParseIP("192.168.0.11"),
				Options: dhcpv4.Options{
					uint8(dhcpv4.OptionDomainName): []byte("domain.tld"),
				},
			},
		},
	}

	hostname, domainname, addr, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("talos-192-168-0-11", hostname)
	suite.Assert().Equal("domain.tld", domainname)
	suite.Assert().Equal(addr, net.ParseIP("192.168.0.11"))
}

func sampleConfigFile() config.Provider {
	return &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NameServers:     []string{"1.2.3.4", "2.3.4.5"},
				NetworkHostname: "myhostname",
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceCIDR:      "192.168.0.10/24",
						DeviceMTU:       9100,
					},
					{
						DeviceInterface: "bond0",
						DeviceCIDR:      "192.168.0.10/24",
						DeviceBond: &v1alpha1.Bond{
							BondInterfaces: []string{"lo"},
							BondMode:       "balance-rr",
						},
					},
				},
			},
		},
	}
}

func dhcpConfigFile() config.Provider {
	return &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
					},
					{
						DeviceInterface: "eth1",
						DeviceCIDR:      "192.168.0.10/24",
						DeviceMTU:       9100,
					},
				},
			},
		},
	}
}
