// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type RouterAdvertisementControllerSuite struct {
	ctest.DefaultSuite
}

func (suite *RouterAdvertisementControllerSuite) TestResolvesLinkAliasAndCleansUp() {
	config := network.NewBGPInstanceConfig("fabric")
	config.TypedSpec().Neighbors = []network.BGPNeighborConfigSpec{{Link: "fabric0"}}
	suite.Create(config)

	link := network.NewLinkStatus(network.NamespaceName, "lo")
	link.TypedSpec().Index = 1
	link.TypedSpec().AltNames = []string{"fabric0"}
	suite.Create(link)

	id := kernel.Sysctl + ".net/ipv6/conf/lo/accept_ra"
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), id, func(res *runtimeres.KernelParamDefaultSpec, assertions *assert.Assertions) {
		assertions.Equal("2", res.TypedSpec().Value)
		assertions.True(res.TypedSpec().IgnoreErrors)
	}, rtestutils.WithNamespace(runtimeres.NamespaceName))

	suite.Destroy(link)
	rtestutils.AssertNoResource[*runtimeres.KernelParamDefaultSpec](
		suite.Ctx(),
		suite.T(),
		suite.State(),
		id,
		rtestutils.WithNamespace(runtimeres.NamespaceName),
	)
}

func TestRouterAdvertisementControllerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RouterAdvertisementControllerSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.RouterAdvertisementController{}))
			},
		},
	})
}
