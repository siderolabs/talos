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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type TimeServerSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *TimeServerSpecSuite) TestSpec() {
	spec := network.NewTimeServerSpec(network.NamespaceName, "timeservers")
	*spec.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{constants.DefaultNTPServer},
		ConfigLayer: network.ConfigDefault,
	}

	suite.Create(spec)

	ctest.AssertResource(
		suite,
		"timeservers",
		func(status *network.TimeServerStatus, asrt *assert.Assertions) {
			asrt.Equal([]string{constants.DefaultNTPServer}, status.TypedSpec().NTPServers)
		},
	)
}

func TestTimeServerSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &TimeServerSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.TimeServerSpecController{}))
			},
		},
	})
}
