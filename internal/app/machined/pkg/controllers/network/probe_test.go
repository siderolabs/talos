// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	networkctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ProbeSuite struct {
	ctest.DefaultSuite
}

func (suite *ProbeSuite) TestReconcile() {
	googleProbeSpec := network.ProbeSpecSpec{
		Interval: 100 * time.Millisecond,
		TCP: network.TCPProbeSpec{
			Endpoint: "google.com:80",
			Timeout:  5 * time.Second,
		},
	}
	googleProbeSpecID, err := googleProbeSpec.ID()
	suite.Require().NoError(err)

	probeGoogle := network.NewProbeSpec(network.NamespaceName, googleProbeSpecID)
	*probeGoogle.TypedSpec() = googleProbeSpec
	suite.Require().NoError(suite.State().Create(suite.Ctx(), probeGoogle))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{googleProbeSpecID}, func(r *network.ProbeStatus, assert *assert.Assertions) {
		assert.Equal(network.ProbeStatusSpec{
			Success: true,
		}, *r.TypedSpec())
	})

	failingProbeSpec := network.ProbeSpecSpec{
		Interval:         100 * time.Millisecond,
		FailureThreshold: 1,
		TCP: network.TCPProbeSpec{
			Endpoint: "google.com:81",
			Timeout:  time.Second,
		},
	}
	failingProbeSpecID, err := failingProbeSpec.ID()
	suite.Require().NoError(err)

	probeFailing := network.NewProbeSpec(network.NamespaceName, failingProbeSpecID)
	*probeFailing.TypedSpec() = failingProbeSpec
	suite.Require().NoError(suite.State().Create(suite.Ctx(), probeFailing))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{failingProbeSpecID}, func(r *network.ProbeStatus, assert *assert.Assertions) {
		assert.False(r.TypedSpec().Success)
	})

	probeFailing.TypedSpec().TCP.Endpoint = "google.com:443"
	suite.Require().NoError(suite.State().Update(suite.Ctx(), probeFailing))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{failingProbeSpecID}, func(r *network.ProbeStatus, assert *assert.Assertions) {
		assert.Equal(network.ProbeStatusSpec{
			Success: true,
		}, *r.TypedSpec())
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), probeFailing.Metadata()))
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), probeGoogle.Metadata()))

	rtestutils.AssertNoResource[*network.ProbeStatus](suite.Ctx(), suite.T(), suite.State(), failingProbeSpecID)
	rtestutils.AssertNoResource[*network.ProbeStatus](suite.Ctx(), suite.T(), suite.State(), googleProbeSpecID)
}

// TestProbeSuite runs the ProbeSuite.
func TestProbeSuite(t *testing.T) {
	suite.Run(t, &ProbeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 20 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&networkctrl.ProbeController{}))
			},
		},
	})
}
