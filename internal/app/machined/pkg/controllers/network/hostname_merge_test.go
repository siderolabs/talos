// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HostnameMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *HostnameMergeSuite) assertHostnames(requiredIDs []string, check func(*network.HostnameSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *HostnameMergeSuite) TestMerge() {
	def := network.NewHostnameSpec(network.ConfigNamespaceName, "default/hostname")
	*def.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "foo",
		Domainname:  "tld",
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewHostnameSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "eth-0",
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewHostnameSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "eth-1",
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewHostnameSpec(network.ConfigNamespaceName, "configuration/hostname")
	*static.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "bar",
		Domainname:  "com",
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Create(res)
	}

	suite.assertHostnames(
		[]string{
			"hostname",
		}, func(r *network.HostnameSpec, asrt *assert.Assertions) {
			asrt.Equal("bar.com", r.TypedSpec().FQDN())
			asrt.Equal("bar", r.TypedSpec().Hostname)
			asrt.Equal("com", r.TypedSpec().Domainname)
		},
	)

	suite.Destroy(static)

	suite.assertHostnames(
		[]string{
			"hostname",
		}, func(r *network.HostnameSpec, asrt *assert.Assertions) {
			asrt.Equal("eth-0", r.TypedSpec().FQDN())
		},
	)
}

func TestHostnameMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &HostnameMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(netctrl.NewHostnameMergeController()))
			},
		},
	})
}
