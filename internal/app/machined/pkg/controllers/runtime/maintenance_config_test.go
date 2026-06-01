// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

func TestMaintenanceConfigSuite(t *testing.T) {
	suite.Run(t, &MaintenanceConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.MaintenanceConfigController{}))
			},
		},
	})
}

type MaintenanceConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *MaintenanceConfigSuite) TestReconcile() {
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{runtime.MaintenanceServiceConfigID},
		func(cfg *runtime.MaintenanceServiceConfig, asrt *assert.Assertions) {
			asrt.Equal(":50000", cfg.TypedSpec().ListenAddress)
			asrt.Nil(cfg.TypedSpec().ReachableAddresses)
		})

	siderolinkConfig := siderolink.NewConfig(config.NamespaceName, siderolink.ConfigID)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), siderolinkConfig))

	rtestutils.AssertNoResource[*runtime.MaintenanceServiceConfig](suite.Ctx(), suite.T(), suite.State(), runtime.MaintenanceServiceConfigID)

	nodeAddresses := network.NewNodeAddress(network.NamespaceName, network.NodeAddressCurrentID)
	nodeAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("172.16.0.1/24"),
		netip.MustParsePrefix("fdae:41e4:649b:9303:2a07:9c7:5b08:aef7/64"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodeAddresses))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{runtime.MaintenanceServiceConfigID},
		func(cfg *runtime.MaintenanceServiceConfig, asrt *assert.Assertions) {
			asrt.Equal("[fdae:41e4:649b:9303:2a07:9c7:5b08:aef7]:50000", cfg.TypedSpec().ListenAddress)
			asrt.Equal([]netip.Addr{netip.MustParseAddr("fdae:41e4:649b:9303:2a07:9c7:5b08:aef7")}, cfg.TypedSpec().ReachableAddresses)
		})
}
