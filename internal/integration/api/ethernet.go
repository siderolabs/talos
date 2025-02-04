// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	networkconfig "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EthernetSuite ...
type EthernetSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EthernetSuite) SuiteName() string {
	return "api.EthernetSuite"
}

// SetupTest ...
func (suite *EthernetSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping ethernet test since provisioner is not qemu")
	}
}

// TearDownTest ...
func (suite *EthernetSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestEthernetConfig verifies changing Ethernet settings.
func (suite *EthernetSuite) TestEthernetConfig() {
	// pick up a random node to test the Ethernet on, and use it throughout the test
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Logf("testing Ethernet on node %s", node)

	// build a Talos API context which is tied to the node
	nodeCtx := client.WithNode(suite.ctx, node)

	// pick a Ethernet links
	ethStatuses, err := safe.StateListAll[*network.EthernetStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var (
		linkName       string
		prevRingConfig *network.EthernetRingsStatus
	)

	for ethStatus := range ethStatuses.All() {
		if ethStatus.TypedSpec().Rings != nil && ethStatus.TypedSpec().Rings.RXMax != nil {
			linkName = ethStatus.Metadata().ID()
			prevRingConfig = ethStatus.TypedSpec().Rings

			marshaled, err := resource.MarshalYAML(ethStatus)
			suite.Require().NoError(err)

			out, err := yaml.Marshal(marshaled)
			suite.Require().NoError(err)

			suite.T().Logf("found link %s with: %s", linkName, string(out))

			break
		}
	}

	suite.Require().NotEmpty(linkName, "no link provides RX rings")

	if os.Getenv("CI") != "" {
		suite.T().Skip("skipping ethtool test in CI, as QEMU version doesn't support updating RX rings for virtio")
	}

	// first, adjust RX rings to be 50% of what it was before
	newRX := pointer.SafeDeref(prevRingConfig.RXMax) / 2

	cfgDocument := networkconfig.NewEthernetConfigV1Alpha1(linkName)
	cfgDocument.RingsConfig = &networkconfig.EthernetRingsConfig{
		RX: pointer.To(newRX),
	}
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	// now EthernetStatus should reflect the new RX rings
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
		func(ethStatus *network.EthernetStatus, asrt *assert.Assertions) {
			asrt.Equal(newRX, pointer.SafeDeref(ethStatus.TypedSpec().Rings.RX))
		},
	)

	// now, let's revert the RX rings to what it was before
	cfgDocument.RingsConfig.RX = prevRingConfig.RX

	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	// now EthernetStatus should reflect the new RX rings
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
		func(ethStatus *network.EthernetStatus, asrt *assert.Assertions) {
			asrt.Equal(pointer.SafeDeref(prevRingConfig.RX), pointer.SafeDeref(ethStatus.TypedSpec().Rings.RX))
		},
	)

	// remove the config document
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)
}

func init() {
	allSuites = append(allSuites, new(EthernetSuite))
}
