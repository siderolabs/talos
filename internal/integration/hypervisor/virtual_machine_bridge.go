// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// TestBridgeAttachment verifies that a bridge-attached interface adds a tap port to the bridge,
// and that the port survives a change of the bridge's settings.
func (suite *LibvirtSuite) TestBridgeAttachment() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)
	name := "vm-bridge-" + uuid.NewString()
	bridgeName := "brvmtest"

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	bridge := networkcfg.NewBridgeConfigV1Alpha1(bridgeName)
	bridge.LinkUp = new(true)

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{
			InterfaceName:    "net0",
			InterfaceLink:    bridgeName,
			AttachmentConfig: hypervisorhelpers.VirtualMachineInterfaceAttachmentBridge,
		},
	}

	// Register cleanup before applying configuration: a failed apply can still leave a guest
	// behind. The guest goes first, so its tap is gone before the bridge is.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		ctx = client.WithNode(ctx, node)

		suite.RemoveMachineConfigDocumentsByName(ctx, hypervisorcfg.VirtualMachineConfigKind, name)
		suite.RemoveMachineConfigDocumentsByName(ctx, networkcfg.BridgeKind, bridgeName)
	})

	suite.PatchMachineConfig(nodeCtx, bridge, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 0, 1)

	var bridgeIndex uint32

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), bridgeName,
		func(link *network.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal(network.LinkKindBridge, link.TypedSpec().Kind)

			bridgeIndex = link.TypedSpec().Index
		},
	)

	tapEnslaved := func() bool {
		links, err := safe.StateListAll[*network.LinkStatus](nodeCtx, suite.Client.State())
		suite.Require().NoError(err)

		for link := range links.All() {
			if link.TypedSpec().Kind == "tun" && link.TypedSpec().MasterIndex == bridgeIndex {
				return true
			}
		}

		return false
	}

	suite.Require().Eventually(tapEnslaved, time.Minute, time.Second, "no tap port on bridge %q", bridgeName)

	// A settings change briefly unslaves every port; the tap must be put back.
	bridge.BridgeSTP.BridgeSTPEnabled = new(true)
	suite.PatchMachineConfig(nodeCtx, bridge)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), bridgeName,
		func(link *network.LinkStatus, asrt *assert.Assertions) {
			asrt.True(link.TypedSpec().BridgeMaster.STP.Enabled)
		},
	)

	suite.Require().Eventually(tapEnslaved, 30*time.Second, time.Second, "tap port left bridge %q after a settings change", bridgeName)
	suite.Require().Never(func() bool { return !tapEnslaved() }, 5*time.Second, 250*time.Millisecond,
		"tap port left bridge %q after a settings change", bridgeName)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.State(), name)
	suite.assertNoDomain(node, name)
}
