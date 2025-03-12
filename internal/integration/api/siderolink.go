// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	siderolinkconfig "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

// SideroLinkSuite ...
type SideroLinkSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *SideroLinkSuite) SuiteName() string {
	return "api.SideroLinkSuite"
}

// SetupTest ...
func (suite *SideroLinkSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
}

// TearDownTest ...
func (suite *SideroLinkSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestTunnelSettingFlip enables/disables the tunnel-over-GRPC setting of the link.
func (suite *SideroLinkSuite) TestTunnelSettingFlip() {
	// pick up a random node to test the SideroLink on, and use it throughout the test
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Logf("testing SideroLink on node %s", node)

	// build a Talos API context which is tied to the node
	nodeCtx := client.WithNode(suite.ctx, node)

	// check if SideroLink is enabled
	sideroLinkConfig, err := safe.StateGetByID[*siderolink.Config](nodeCtx, suite.Client.COSI, siderolink.ConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			suite.T().Skip("skipping the test since SideroLink is not enabled")
		}

		suite.Require().NoError(err)
	}

	// assert that siderolink is connected
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, siderolink.StatusID, func(status *siderolink.Status, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Connected, "SideroLink is not connected")
	})

	apiURL, err := url.Parse(sideroLinkConfig.TypedSpec().APIEndpoint)
	suite.Require().NoError(err)

	q := apiURL.Query()

	// flip the tunnel setting
	if sideroLinkConfig.TypedSpec().Tunnel {
		q.Del("grpc_tunnel")

		suite.T().Log("flipping the tunnel setting to false")
	} else {
		q.Set("grpc_tunnel", "true")

		suite.T().Log("flipping the tunnel setting to true")
	}

	apiURL.RawQuery = q.Encode()

	cfgDocument := siderolinkconfig.NewConfigV1Alpha1()
	cfgDocument.APIUrlConfig.URL = apiURL

	// patch settings
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	// first, the config should be updated
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, siderolink.ConfigID, func(config *siderolink.Config, asrt *assert.Assertions) {
		asrt.Equal(!sideroLinkConfig.TypedSpec().Tunnel, config.TypedSpec().Tunnel, "SideroLink tunnel setting is not updated")
	})

	suite.T().Log("configuration updated, waiting for SideroLink to reconnect...")

	// second, new status should reflect the change
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, siderolink.StatusID, func(status *siderolink.Status, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Connected, "SideroLink is not connected")
		asrt.Equal(!sideroLinkConfig.TypedSpec().Tunnel, status.TypedSpec().GRPCTunnel, "SideroLink tunnel setting is not updated")
	})
}

func init() {
	allSuites = append(allSuites, new(SideroLinkSuite))
}
