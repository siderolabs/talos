// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type StatusSuite struct {
	ctest.DefaultSuite
}

func (suite *StatusSuite) TestNone() {
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{network.StatusID}, func(r *network.Status, assert *assert.Assertions) {
		assert.Equal(network.StatusSpec{}, *r.TypedSpec())
	})
}

func (suite *StatusSuite) TestAddresses() {
	nodeAddress := network.NewNodeAddress(network.NamespaceName, network.NodeAddressCurrentID)
	nodeAddress.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24")}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodeAddress))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{network.StatusID}, func(r *network.Status, assert *assert.Assertions) {
		assert.Equal(network.StatusSpec{AddressReady: true}, *r.TypedSpec())
	})
}

func (suite *StatusSuite) TestRoutes() {
	route := network.NewRouteStatus(network.NamespaceName, "foo")
	route.TypedSpec().Gateway = netip.MustParseAddr("10.0.0.1")

	suite.Require().NoError(suite.State().Create(suite.Ctx(), route))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{network.StatusID}, func(r *network.Status, assert *assert.Assertions) {
		assert.Equal(network.StatusSpec{ConnectivityReady: true}, *r.TypedSpec())
	})
}

func (suite *StatusSuite) TestProbeStatuses() {
	probeStatus := network.NewProbeStatus(network.NamespaceName, "foo")
	probeStatus.TypedSpec().Success = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), probeStatus))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{network.StatusID}, func(r *network.Status, assert *assert.Assertions) {
		assert.Equal(network.StatusSpec{ConnectivityReady: true}, *r.TypedSpec())
	})

	// failing probe make status not ready
	route := network.NewRouteStatus(network.NamespaceName, "foo")
	route.TypedSpec().Gateway = netip.MustParseAddr("10.0.0.1")

	suite.Require().NoError(suite.State().Create(suite.Ctx(), route))

	probeStatusFail := network.NewProbeStatus(network.NamespaceName, "failing")
	suite.Require().NoError(suite.State().Create(suite.Ctx(), probeStatusFail))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{network.StatusID}, func(r *network.Status, assert *assert.Assertions) {
		assert.Equal(network.StatusSpec{}, *r.TypedSpec())
	})
}

func (suite *StatusSuite) TestHostname() {
	hostname := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostname.TypedSpec().Hostname = "foo"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), hostname))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{network.StatusID}, func(r *network.Status, assert *assert.Assertions) {
		assert.Equal(network.StatusSpec{HostnameReady: true}, *r.TypedSpec())
	})
}

func (suite *StatusSuite) TestEtcFiles() {
	for _, f := range []string{"hosts", "resolv.conf"} {
		suite.Require().NoError(suite.State().Create(suite.Ctx(), files.NewEtcFileStatus(files.NamespaceName, f)))
	}

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{network.StatusID}, func(r *network.Status, assert *assert.Assertions) {
		assert.Equal(network.StatusSpec{EtcFilesReady: true}, *r.TypedSpec())
	})
}

func TestStatusSuite(t *testing.T) {
	suite.Run(t, &StatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(
					&netctrl.StatusController{
						V1Alpha1Mode: runtime.ModeMetal,
					},
				))
			},
		},
	})
}
