// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type AddressStatusSuite struct {
	ctest.DefaultSuite
}

func (suite *AddressStatusSuite) TestLoopback() {
	ctest.AssertResource(suite, "lo/127.0.0.1/8", func(r *network.AddressStatus, asrt *assert.Assertions) {})
}

func TestAddressStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &AddressStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.AddressStatusController{}))
			},
		},
	})
}
