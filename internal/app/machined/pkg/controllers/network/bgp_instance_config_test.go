// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type BGPInstanceConfigSuite struct {
	ctest.DefaultSuite
}

func newFabricConfig(holdTime time.Duration) *networkcfg.BGPInstanceConfigV1Alpha1 {
	instance := networkcfg.NewBGPInstanceConfigV1Alpha1("fabric")
	instance.BGPLocalASN = 65001
	instance.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	instance.BGPRouteSource = meta.Addr{Addr: netip.MustParseAddr("10.0.0.2")}
	instance.BGPAdvertise = []string{"node-ip"}
	instance.BGPMultipath = true
	instance.BGPMaxPaths = 4
	instance.BGPNeighborConfigs = []networkcfg.BGPNeighborConfig{
		{
			NeighborLinkConfig: "fabric0",
			NeighborPeerASN:    65002,
			NeighborHoldTime:   holdTime,
			NeighborBFDConfig: &networkcfg.BGPBFDConfig{
				BFDTransmitInterval: 300 * time.Millisecond,
				BFDReceiveInterval:  400 * time.Millisecond,
				BFDDetectMultiplier: 3,
			},
		},
		{
			NeighborAddressConfig: meta.Addr{Addr: netip.MustParseAddr("192.0.2.1")},
			NeighborPeerASN:       65003,
			NeighborLocalASN:      65004,
			NeighborPassive:       true,
			NeighborHoldTime:      15 * time.Second,
		},
	}

	return instance
}

func (suite *BGPInstanceConfigSuite) TestRenderInlineBehaviorAndPreserveConfiguredNames() {
	instance := newFabricConfig(9 * time.Second)
	ctr, err := container.New(instance)
	suite.Require().NoError(err)

	machineConfig := configresource.NewMachineConfig(ctr)
	suite.Create(machineConfig)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric", func(res *network.BGPInstanceConfig, assertions *assert.Assertions) {
		spec := res.TypedSpec()
		assertions.Equal(uint32(65001), spec.LocalASN)
		assertions.Equal(netip.MustParseAddr("10.0.0.1"), spec.RouterID)
		assertions.Equal(netip.MustParseAddr("10.0.0.2"), spec.RouteSource)
		assertions.Equal([]string{"node-ip"}, spec.AdvertiseLinks)
		assertions.Equal(nethelpers.TableMain, spec.VRFTable)
		assertions.Len(spec.Neighbors, 2)
		assertions.Equal("fabric0", spec.Neighbors[0].Link)
		assertions.Equal(9*time.Second, spec.Neighbors[0].HoldTime)
		assertions.NotNil(spec.Neighbors[0].BFD)
		assertions.Equal(300*time.Millisecond, spec.Neighbors[0].BFD.TransmitInterval)
		assertions.Equal(400*time.Millisecond, spec.Neighbors[0].BFD.ReceiveInterval)
		assertions.Equal(uint8(3), spec.Neighbors[0].BFD.DetectMultiplier)
		assertions.Equal(uint32(65004), spec.Neighbors[1].LocalASN)
		assertions.True(spec.Neighbors[1].Passive)
		assertions.Equal(15*time.Second, spec.Neighbors[1].HoldTime)
		assertions.Nil(spec.Neighbors[1].BFD)
	}, rtestutils.WithNamespace(network.NamespaceName))

	updatedInstance := newFabricConfig(12 * time.Second)
	updatedContainer, err := container.New(updatedInstance)
	suite.Require().NoError(err)

	updatedMachineConfig := configresource.NewMachineConfig(updatedContainer)
	updatedMachineConfig.Metadata().SetVersion(machineConfig.Metadata().Version())
	suite.Update(updatedMachineConfig)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric", func(res *network.BGPInstanceConfig, assertions *assert.Assertions) {
		assertions.Equal(12*time.Second, res.TypedSpec().Neighbors[0].HoldTime)
	}, rtestutils.WithNamespace(network.NamespaceName))

	suite.Destroy(updatedMachineConfig)
	rtestutils.AssertNoResource[*network.BGPInstanceConfig](suite.Ctx(), suite.T(), suite.State(), "fabric", rtestutils.WithNamespace(network.NamespaceName))
}

func (suite *BGPInstanceConfigSuite) TestVRFResolutionAndMembership() {
	vrf := networkcfg.NewVRFConfigV1Alpha1("vrf-blue")
	vrf.VRFTable = 88
	vrf.VRFLinks = []string{"eth1"}

	instance := networkcfg.NewBGPInstanceConfigV1Alpha1("blue")
	instance.BGPVRF = "vrf-blue"
	instance.BGPLocalASN = 65001
	instance.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	instance.BGPNeighborConfigs = []networkcfg.BGPNeighborConfig{{NeighborLinkConfig: "eth1"}}

	ctr, err := container.New(vrf, instance)
	suite.Require().NoError(err)
	suite.Create(configresource.NewMachineConfig(ctr))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "blue", func(res *network.BGPInstanceConfig, assertions *assert.Assertions) {
		assertions.Equal("vrf-blue", res.TypedSpec().VRF)
		assertions.Equal(nethelpers.RoutingTable(88), res.TypedSpec().VRFTable)
	}, rtestutils.WithNamespace(network.NamespaceName))
}

func (suite *BGPInstanceConfigSuite) TestInvalidProjectionRetainsLastKnownGood() {
	instance := newFabricConfig(9 * time.Second)
	ctr, err := container.New(instance)
	suite.Require().NoError(err)

	machineConfig := configresource.NewMachineConfig(ctr)
	suite.Create(machineConfig)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric", func(res *network.BGPInstanceConfig, assertions *assert.Assertions) {
		assertions.Equal(9*time.Second, res.TypedSpec().Neighbors[0].HoldTime)
	}, rtestutils.WithNamespace(network.NamespaceName))

	invalidInstance := newFabricConfig(12 * time.Second)
	invalidInstance.BGPVRF = "missing-vrf"
	invalidContainer, err := container.New(invalidInstance)
	suite.Require().NoError(err)

	invalidMachineConfig := configresource.NewMachineConfig(invalidContainer)
	invalidMachineConfig.Metadata().SetVersion(machineConfig.Metadata().Version())
	suite.Update(invalidMachineConfig)

	// The projection returns an error before tracking/writing outputs, retaining the last valid set.
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), "fabric", func(res *network.BGPInstanceConfig, assertions *assert.Assertions) {
		assertions.Equal(9*time.Second, res.TypedSpec().Neighbors[0].HoldTime)
	}, rtestutils.WithNamespace(network.NamespaceName))
}

func TestBGPInstanceConfigControllerInputs(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []controller.Input{{
		Namespace: configresource.NamespaceName,
		Type:      configresource.MachineConfigType,
		ID:        optional.Some(configresource.ActiveID),
		Kind:      controller.InputWeak,
	}}, (&netctrl.BGPInstanceConfigController{}).Inputs())
}

func TestBGPInstanceConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &BGPInstanceConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.BGPInstanceConfigController{}))
			},
		},
	})
}
