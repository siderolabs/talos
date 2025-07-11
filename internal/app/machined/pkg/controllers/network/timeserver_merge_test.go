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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type TimeServerMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *TimeServerMergeSuite) assertTimeServers(
	requiredIDs []string,
	check func(*network.TimeServerSpec, *assert.Assertions),
) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *TimeServerMergeSuite) TestMerge() {
	def := network.NewTimeServerSpec(network.ConfigNamespaceName, "default/timeservers")
	*def.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{constants.DefaultNTPServer},
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewTimeServerSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"ntp.eth0"},
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewTimeServerSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"ntp.eth1"},
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewTimeServerSpec(network.ConfigNamespaceName, "configuration/timeservers")
	*static.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"my.ntp"},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Create(res)
	}

	suite.assertTimeServers(
		[]string{
			"timeservers",
		}, func(r *network.TimeServerSpec, asrt *assert.Assertions) {
			asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
		},
	)

	suite.Destroy(static)

	suite.assertTimeServers(
		[]string{
			"timeservers",
		}, func(r *network.TimeServerSpec, asrt *assert.Assertions) {
			asrt.Equal([]string{"ntp.eth0", "ntp.eth1"}, r.TypedSpec().NTPServers)
		},
	)
}

func TestTimeServerMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &TimeServerMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(netctrl.NewTimeServerMergeController()))
			},
		},
	})
}
