// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type DevicesSuite struct {
	ctest.DefaultSuite
}

func TestDevicesSuite(t *testing.T) {
	suite.Run(t, new(DevicesSuite))
}

func (suite *DevicesSuite) TestDiscover() {
	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DevicesController{}))

	// these devices should always exist on Linux
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"loop0", "loop1"}, func(r *block.Device, assertions *assert.Assertions) {
		assertions.Equal("disk", r.TypedSpec().Type)
	})
}
