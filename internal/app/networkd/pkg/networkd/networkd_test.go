// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"net"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
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
	nwd, err := New(sampleConfigFile())
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
		address      net.IP
		domainname   string
		err          error
		hostname     string
		nwd          *Networkd
		sampleConfig runtime.Configurator
	)

	nwd, err = New(nil)
	suite.Require().NoError(err)

	hostname, _, address, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("talos-127-0-1-1", hostname)
	suite.Assert().Equal(address, net.ParseIP("127.0.1.1"))

	sampleConfig = sampleConfigFile()

	nwd, err = New(sampleConfig)
	suite.Require().NoError(err)

	hostname, _, address, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("myhostname", hostname)
	suite.Assert().Equal(address, net.ParseIP("192.168.0.10"))

	sampleConfig.Machine().Network().SetHostname("")

	nwd, err = New(sampleConfig)
	suite.Require().NoError(err)

	hostname, _, address, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("talos-192-168-0-10", hostname)
	suite.Assert().Equal(address, net.ParseIP("192.168.0.10"))

	sampleConfig.Machine().Network().SetHostname("somereallyreallyreallylongstringthathasmorethan63charactersbecauseweneedtotestit")

	nwd, err = New(sampleConfig)
	suite.Require().NoError(err)

	// nolint: dogsled
	_, _, _, err = nwd.decideHostname()
	suite.Require().Error(err)

	sampleConfig.Machine().Network().SetHostname("dadjokes.biz.dev.com.org.io")

	nwd, err = New(sampleConfig)
	suite.Require().NoError(err)

	hostname, domainname, _, err = nwd.decideHostname()
	suite.Require().NoError(err)
	suite.Assert().Equal("dadjokes", hostname)
	suite.Assert().Equal("biz.dev.com.org.io", domainname)
}

func sampleConfigFile() runtime.Configurator {
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
					},
					{
						Interface: "bond0",
						CIDR:      "192.168.0.10/24",
						Bond: &machine.Bond{
							Interfaces: []string{"lo"},
							Mode:       "balance-rr",
						},
					},
				},
			},
		},
	}
}
