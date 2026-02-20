// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type LinkSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *LinkSpecSuite) uniqueDummyInterface() string {
	return fmt.Sprintf("dummy%02x%02x%02x", rand.Int32()&0xff, rand.Int32()&0xff, rand.Int32()&0xff)
}

func (suite *LinkSpecSuite) TestLoopback() {
	loopback := network.NewLinkSpec(network.NamespaceName, "lo")
	*loopback.TypedSpec() = network.LinkSpecSpec{
		Name:        "lo",
		Up:          true,
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{loopback} {
		suite.Create(res)
	}

	ctest.AssertResource(suite, "lo", func(r *network.LinkStatus, asrt *assert.Assertions) {})
}

func (suite *LinkSpecSuite) TestDummy() {
	dummyInterface := suite.uniqueDummyInterface()

	dummy := network.NewLinkSpec(network.NamespaceName, dummyInterface)
	*dummy.TypedSpec() = network.LinkSpecSpec{
		Name:        dummyInterface,
		Type:        nethelpers.LinkEther,
		Kind:        "dummy",
		MTU:         1400,
		Up:          true,
		Logical:     true,
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{dummy} {
		suite.Create(res)
	}

	newHardwareAddr := net.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}

	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal("dummy", r.TypedSpec().Kind)
		asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
		asrt.EqualValues(1400, r.TypedSpec().MTU)
		asrt.NotEqual(newHardwareAddr, net.HardwareAddr(r.TypedSpec().HardwareAddr))
	})

	// attempt to change the hardware address
	ctest.UpdateWithConflicts(suite, dummy, func(r *network.LinkSpec) error {
		r.TypedSpec().HardwareAddress = nethelpers.HardwareAddr(newHardwareAddr)

		return nil
	})

	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(newHardwareAddr, net.HardwareAddr(r.TypedSpec().HardwareAddr))
	})

	// check default multicast behavior (disabled on dummy interfaces)
	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(r.TypedSpec().Flags&unix.IFF_MULTICAST == unix.IFF_MULTICAST, false)
	})

	// attempt to change multicast flag
	ctest.UpdateWithConflicts(suite, dummy, func(r *network.LinkSpec) error {
		r.TypedSpec().Multicast = new(true)

		return nil
	})

	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(r.TypedSpec().Flags&unix.IFF_MULTICAST == unix.IFF_MULTICAST, true)
	})

	// attempt to disable multicast
	ctest.UpdateWithConflicts(suite, dummy, func(r *network.LinkSpec) error {
		r.TypedSpec().Multicast = new(bool)
		r.TypedSpec().Multicast = new(false)

		return nil
	})

	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(r.TypedSpec().Flags&unix.IFF_MULTICAST == unix.IFF_MULTICAST, false)
	})

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), dummy.Metadata()))

	ctest.AssertNoResource[*network.LinkSpec](suite, dummyInterface)
}

func (suite *LinkSpecSuite) TestDummyWithMAC() {
	dummyInterface := suite.uniqueDummyInterface()

	newHardwareAddr := net.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}

	dummy := network.NewLinkSpec(network.NamespaceName, dummyInterface)
	*dummy.TypedSpec() = network.LinkSpecSpec{
		Name:            dummyInterface,
		Type:            nethelpers.LinkEther,
		Kind:            "dummy",
		HardwareAddress: nethelpers.HardwareAddr(newHardwareAddr),
		Up:              true,
		Logical:         true,
		ConfigLayer:     network.ConfigDefault,
	}

	for _, res := range []resource.Resource{dummy} {
		suite.Create(res)
	}

	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal("dummy", r.TypedSpec().Kind)
		asrt.Equal(newHardwareAddr, net.HardwareAddr(r.TypedSpec().HardwareAddr))
	})

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), dummy.Metadata()))

	ctest.AssertNoResource[*network.LinkSpec](suite, dummyInterface)
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestVLAN() {
	dummyInterface := suite.uniqueDummyInterface()

	dummy := network.NewLinkSpec(network.NamespaceName, dummyInterface)
	*dummy.TypedSpec() = network.LinkSpecSpec{
		Name:        dummyInterface,
		Type:        nethelpers.LinkEther,
		Kind:        "dummy",
		Up:          true,
		Logical:     true,
		ConfigLayer: network.ConfigDefault,
	}

	vlanName1 := fmt.Sprintf("%s.%d", dummyInterface, 2)
	vlan1 := network.NewLinkSpec(network.NamespaceName, vlanName1)
	*vlan1.TypedSpec() = network.LinkSpecSpec{
		Name:        vlanName1,
		Type:        nethelpers.LinkEther,
		Kind:        network.LinkKindVLAN,
		Up:          true,
		Logical:     true,
		ParentName:  dummyInterface,
		ConfigLayer: network.ConfigDefault,
		VLAN: network.VLANSpec{
			VID:      2,
			Protocol: nethelpers.VLANProtocol8021Q,
		},
	}

	vlanName2 := fmt.Sprintf("%s.%d", dummyInterface, 4)
	vlan2 := network.NewLinkSpec(network.NamespaceName, vlanName2)
	*vlan2.TypedSpec() = network.LinkSpecSpec{
		Name:        vlanName2,
		Type:        nethelpers.LinkEther,
		Kind:        network.LinkKindVLAN,
		Up:          true,
		Logical:     true,
		ParentName:  dummyInterface,
		ConfigLayer: network.ConfigDefault,
		VLAN: network.VLANSpec{
			VID:      4,
			Protocol: nethelpers.VLANProtocol8021Q,
		},
	}

	for _, res := range []resource.Resource{dummy, vlan1, vlan2} {
		suite.Create(res)
	}

	ctest.AssertResources(suite, []string{dummyInterface, vlanName1, vlanName2}, func(r *network.LinkStatus, asrt *assert.Assertions) {
		switch r.Metadata().ID() {
		case dummyInterface:
			asrt.Equal("dummy", r.TypedSpec().Kind)
		case vlanName1, vlanName2:
			asrt.Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
			asrt.Equal(nethelpers.VLANProtocol8021Q, r.TypedSpec().VLAN.Protocol)

			if r.Metadata().ID() == vlanName1 {
				asrt.EqualValues(2, r.TypedSpec().VLAN.VID)
			} else {
				asrt.EqualValues(4, r.TypedSpec().VLAN.VID)
			}
		}

		asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
	})

	// attempt to change VLAN ID
	ctest.UpdateWithConflicts(suite, vlan1, func(r *network.LinkSpec) error {
		r.TypedSpec().VLAN.VID = 42

		return nil
	})

	ctest.AssertResource(suite, vlanName1, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
		asrt.Equal(nethelpers.VLANProtocol8021Q, r.TypedSpec().VLAN.Protocol)
		asrt.EqualValues(42, r.TypedSpec().VLAN.VID)
	})

	// teardown the links
	for _, r := range []resource.Resource{vlan1, vlan2, dummy} {
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), r.Metadata()))
	}

	ctest.AssertNoResource[*network.LinkStatus](suite, dummyInterface)
	ctest.AssertNoResource[*network.LinkStatus](suite, vlanName1)
	ctest.AssertNoResource[*network.LinkStatus](suite, vlanName2)
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestVLANViaAlias() {
	dummyInterface := suite.uniqueDummyInterface()

	dummy := network.NewLinkSpec(network.NamespaceName, dummyInterface)
	*dummy.TypedSpec() = network.LinkSpecSpec{
		Name:        dummyInterface,
		Type:        nethelpers.LinkEther,
		Kind:        "dummy",
		Up:          true,
		Logical:     true,
		ConfigLayer: network.ConfigDefault,
	}

	suite.Create(dummy)

	// create dummy interface, and create an alias for it manually
	ctest.AssertResource(suite, dummyInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal("dummy", r.TypedSpec().Kind)
		asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
	})

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	dummyAlias := suite.uniqueDummyInterface()

	suite.Require().NoError(
		conn.Link.Set(
			&rtnetlink.LinkMessage{
				Index: uint32(iface.Index),
				Attributes: &rtnetlink.LinkAttributes{
					Alias: &dummyAlias,
				},
			},
		),
	)

	vlanName1 := fmt.Sprintf("%s.%d", dummyAlias, 2)
	vlan1 := network.NewLinkSpec(network.NamespaceName, vlanName1)
	*vlan1.TypedSpec() = network.LinkSpecSpec{
		Name:        vlanName1,
		Type:        nethelpers.LinkEther,
		Kind:        network.LinkKindVLAN,
		Up:          true,
		Logical:     true,
		ParentName:  dummyAlias,
		ConfigLayer: network.ConfigDefault,
		VLAN: network.VLANSpec{
			VID:      2,
			Protocol: nethelpers.VLANProtocol8021Q,
		},
	}

	vlanName2 := fmt.Sprintf("%s.%d", dummyAlias, 4)
	vlan2 := network.NewLinkSpec(network.NamespaceName, vlanName2)
	*vlan2.TypedSpec() = network.LinkSpecSpec{
		Name:        vlanName2,
		Type:        nethelpers.LinkEther,
		Kind:        network.LinkKindVLAN,
		Up:          true,
		Logical:     true,
		ParentName:  dummyAlias,
		ConfigLayer: network.ConfigDefault,
		VLAN: network.VLANSpec{
			VID:      4,
			Protocol: nethelpers.VLANProtocol8021Q,
		},
	}

	for _, res := range []resource.Resource{vlan1, vlan2} {
		suite.Create(res)
	}

	ctest.AssertResources(suite, []string{dummyInterface, vlanName1, vlanName2}, func(r *network.LinkStatus, asrt *assert.Assertions) {
		switch r.Metadata().ID() {
		case dummyInterface:
			asrt.Equal("dummy", r.TypedSpec().Kind)
			asrt.Equal(dummyAlias, r.TypedSpec().Alias)
		case vlanName1, vlanName2:
			asrt.Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
			asrt.Equal(nethelpers.VLANProtocol8021Q, r.TypedSpec().VLAN.Protocol)

			if r.Metadata().ID() == vlanName1 {
				asrt.EqualValues(2, r.TypedSpec().VLAN.VID)
			} else {
				asrt.EqualValues(4, r.TypedSpec().VLAN.VID)
			}
		}

		asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
	})

	// teardown the links
	for _, r := range []resource.Resource{vlan1, vlan2, dummy} {
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), r.Metadata()))
	}

	ctest.AssertNoResource[*network.LinkStatus](suite, dummyInterface)
	ctest.AssertNoResource[*network.LinkStatus](suite, vlanName1)
	ctest.AssertNoResource[*network.LinkStatus](suite, vlanName2)
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestBond() {
	bondName := suite.uniqueDummyInterface()
	bond := network.NewLinkSpec(network.NamespaceName, bondName)
	*bond.TypedSpec() = network.LinkSpecSpec{
		Name:    bondName,
		Type:    nethelpers.LinkEther,
		Kind:    network.LinkKindBond,
		Up:      true,
		Logical: true,
		BondMaster: network.BondMasterSpec{
			Mode:            nethelpers.BondModeActiveBackup,
			ARPAllTargets:   nethelpers.ARPAllTargetsAll,
			PrimaryReselect: nethelpers.PrimaryReselectBetter,
			FailOverMac:     nethelpers.FailOverMACFollow,
			ADSelect:        nethelpers.ADSelectBandwidth,
			MIIMon:          100,
			DownDelay:       100,
			ResendIGMP:      2,
			UseCarrier:      true,
		},
		ConfigLayer: network.ConfigDefault,
	}
	networkadapter.BondMasterSpec(&bond.TypedSpec().BondMaster).FillDefaults()

	dummy0Name := suite.uniqueDummyInterface()
	dummy0 := network.NewLinkSpec(network.NamespaceName, dummy0Name)
	*dummy0.TypedSpec() = network.LinkSpecSpec{
		Name:    dummy0Name,
		Type:    nethelpers.LinkEther,
		Kind:    "dummy",
		Up:      true,
		Logical: true,
		BondSlave: network.BondSlave{
			MasterName: bondName,
			SlaveIndex: 0,
		},
		ConfigLayer: network.ConfigDefault,
	}

	dummy1Name := suite.uniqueDummyInterface()
	dummy1 := network.NewLinkSpec(network.NamespaceName, dummy1Name)
	*dummy1.TypedSpec() = network.LinkSpecSpec{
		Name:    dummy1Name,
		Type:    nethelpers.LinkEther,
		Kind:    "dummy",
		Up:      true,
		Logical: true,
		BondSlave: network.BondSlave{
			MasterName: bondName,
			SlaveIndex: 1,
		},
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{dummy0, dummy1, bond} {
		suite.Create(res)
	}

	ctest.AssertResources(suite, []string{dummy0Name, dummy1Name, bondName}, func(r *network.LinkStatus, asrt *assert.Assertions) {
		switch r.Metadata().ID() {
		case bondName:
			asrt.Equal(network.LinkKindBond, r.TypedSpec().Kind)
			asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
		case dummy0Name, dummy1Name:
			asrt.Equal("dummy", r.TypedSpec().Kind)
			asrt.Equal(nethelpers.OperStateUnknown, r.TypedSpec().OperationalState)
			asrt.NotZero(r.TypedSpec().MasterIndex)
		}
	})

	// attempt to change bond type
	ctest.UpdateWithConflicts(suite, bond, func(r *network.LinkSpec) error {
		r.TypedSpec().BondMaster.Mode = nethelpers.BondModeRoundrobin

		return nil
	})

	ctest.AssertResource(suite, bondName, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(nethelpers.BondModeRoundrobin, r.TypedSpec().BondMaster.Mode)
	})

	// unslave one of the interfaces
	ctest.UpdateWithConflicts(suite, dummy0, func(r *network.LinkSpec) error {
		r.TypedSpec().BondSlave.MasterName = ""

		return nil
	})

	ctest.AssertResource(suite, dummy0Name, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Zero(r.TypedSpec().MasterIndex)
	})

	// teardown the links
	for _, r := range []resource.Resource{dummy0, dummy1, bond} {
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), r.Metadata()))
	}

	ctest.AssertNoResource[*network.LinkStatus](suite, dummy0Name)
	ctest.AssertNoResource[*network.LinkStatus](suite, dummy1Name)
	ctest.AssertNoResource[*network.LinkStatus](suite, bondName)
}

func (suite *LinkSpecSuite) TestBondActiveBackup() {
	bondName := suite.uniqueDummyInterface()
	bond := network.NewLinkSpec(network.NamespaceName, bondName)
	*bond.TypedSpec() = network.LinkSpecSpec{
		Name:    bondName,
		Type:    nethelpers.LinkEther,
		Kind:    network.LinkKindBond,
		Up:      true,
		Logical: true,
		BondMaster: network.BondMasterSpec{
			Mode:            nethelpers.BondModeActiveBackup,
			HashPolicy:      nethelpers.BondXmitPolicyLayer2,
			LACPRate:        nethelpers.LACPRateSlow,
			ARPValidate:     nethelpers.ARPValidateNone,
			ARPAllTargets:   nethelpers.ARPAllTargetsAny,
			PrimaryReselect: nethelpers.PrimaryReselectAlways,
			FailOverMac:     nethelpers.FailOverMACNone,
		},
		ConfigLayer: network.ConfigDefault,
	}

	networkadapter.BondMasterSpec(&bond.TypedSpec().BondMaster).FillDefaults()

	for idx := range 2 {
		dummyName := suite.uniqueDummyInterface()
		dummy := network.NewLinkSpec(network.NamespaceName, dummyName)
		*dummy.TypedSpec() = network.LinkSpecSpec{
			Name:    dummyName,
			Type:    nethelpers.LinkEther,
			Kind:    "dummy",
			Up:      true,
			Logical: true,
			BondSlave: network.BondSlave{
				MasterName: bondName,
				SlaveIndex: idx,
			},
			ConfigLayer: network.ConfigDefault,
		}
		suite.Create(dummy)
	}

	suite.Create(bond)

	ctest.AssertResource(suite, bondName, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(network.LinkKindBond, r.TypedSpec().Kind)
		asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
	})
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestBond8023ad() {
	bondName := suite.uniqueDummyInterface()
	bond := network.NewLinkSpec(network.NamespaceName, bondName)
	*bond.TypedSpec() = network.LinkSpecSpec{
		Name:    bondName,
		Type:    nethelpers.LinkEther,
		Kind:    network.LinkKindBond,
		MTU:     9000,
		Up:      true,
		Logical: true,
		BondMaster: network.BondMasterSpec{
			Mode:       nethelpers.BondMode8023AD,
			LACPRate:   nethelpers.LACPRateFast,
			UseCarrier: true,
		},
		ConfigLayer: network.ConfigDefault,
	}
	networkadapter.BondMasterSpec(&bond.TypedSpec().BondMaster).FillDefaults()

	//nolint:prealloc
	var (
		dummies    []resource.Resource
		dummyNames []string
	)

	for range 4 {
		dummyName := suite.uniqueDummyInterface()
		dummy := network.NewLinkSpec(network.NamespaceName, dummyName)
		*dummy.TypedSpec() = network.LinkSpecSpec{
			Name:    dummyName,
			Type:    nethelpers.LinkEther,
			Kind:    "dummy",
			Up:      true,
			Logical: true,
			BondSlave: network.BondSlave{
				MasterName: bondName,
				SlaveIndex: 0,
			},
			ConfigLayer: network.ConfigDefault,
		}

		dummies = append(dummies, dummy)
		dummyNames = append(dummyNames, dummyName)
	}

	for _, res := range append(dummies, bond) {
		suite.Create(res)
	}

	ctest.AssertResources(suite, slices.Concat(dummyNames, []string{bondName}), func(r *network.LinkStatus, asrt *assert.Assertions) {
		switch r.Metadata().ID() {
		case bondName:
			asrt.Equal(network.LinkKindBond, r.TypedSpec().Kind)
			asrt.EqualValues(9000, r.TypedSpec().MTU)
			asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
		default:
			asrt.Equal("dummy", r.TypedSpec().Kind)
			asrt.Equal(nethelpers.OperStateUnknown, r.TypedSpec().OperationalState)
			asrt.NotZero(r.TypedSpec().MasterIndex)
		}
	})

	// teardown the links
	for _, r := range append(dummies, bond) {
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), r.Metadata()))
	}

	ctest.AssertNoResource[*network.LinkStatus](suite, bondName)

	for _, n := range dummyNames {
		ctest.AssertNoResource[*network.LinkStatus](suite, n)
	}
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestBridge() {
	bridgeName := suite.uniqueDummyInterface()
	bridge := network.NewLinkSpec(network.NamespaceName, bridgeName)
	*bridge.TypedSpec() = network.LinkSpecSpec{
		Name:    bridgeName,
		Type:    nethelpers.LinkEther,
		Kind:    network.LinkKindBridge,
		Up:      true,
		Logical: true,
		BridgeMaster: network.BridgeMasterSpec{
			STP: network.STPSpec{
				Enabled: false,
			},
		},
		ConfigLayer: network.ConfigDefault,
	}

	dummy0Name := suite.uniqueDummyInterface()
	dummy0 := network.NewLinkSpec(network.NamespaceName, dummy0Name)
	*dummy0.TypedSpec() = network.LinkSpecSpec{
		Name:    dummy0Name,
		Type:    nethelpers.LinkEther,
		Kind:    "dummy",
		Up:      true,
		Logical: true,
		BridgeSlave: network.BridgeSlave{
			MasterName: bridgeName,
		},
		ConfigLayer: network.ConfigDefault,
	}

	dummy1Name := suite.uniqueDummyInterface()
	dummy1 := network.NewLinkSpec(network.NamespaceName, dummy1Name)
	*dummy1.TypedSpec() = network.LinkSpecSpec{
		Name:    dummy1Name,
		Type:    nethelpers.LinkEther,
		Kind:    "dummy",
		Up:      true,
		Logical: true,
		BridgeSlave: network.BridgeSlave{
			MasterName: bridgeName,
		},
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{dummy0, dummy1, bridge} {
		suite.Create(res)
	}

	ctest.AssertResources(suite, []string{dummy0Name, dummy1Name, bridgeName}, func(r *network.LinkStatus, asrt *assert.Assertions) {
		switch r.Metadata().ID() {
		case bridgeName:
			asrt.Equal(network.LinkKindBridge, r.TypedSpec().Kind)
			asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
		case dummy0Name, dummy1Name:
			asrt.Equal("dummy", r.TypedSpec().Kind)
			asrt.Equal(nethelpers.OperStateUnknown, r.TypedSpec().OperationalState)
			asrt.NotZero(r.TypedSpec().MasterIndex)
		}
	})

	// attempt to enable STP & VLAN filtering
	ctest.UpdateWithConflicts(suite, bridge, func(r *network.LinkSpec) error {
		r.TypedSpec().BridgeMaster.STP.Enabled = true
		r.TypedSpec().BridgeMaster.VLAN.FilteringEnabled = true

		return nil
	})

	ctest.AssertResource(suite, bridgeName, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(network.LinkKindBridge, r.TypedSpec().Kind)
		asrt.EqualValues(true, r.TypedSpec().BridgeMaster.STP.Enabled)
		asrt.EqualValues(true, r.TypedSpec().BridgeMaster.VLAN.FilteringEnabled)
	})

	// unslave one of the interfaces
	ctest.UpdateWithConflicts(suite, dummy0, func(r *network.LinkSpec) error {
		r.TypedSpec().BridgeSlave.MasterName = ""

		return nil
	})

	ctest.AssertResource(suite, dummy0Name, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Zero(r.TypedSpec().MasterIndex)
	})

	// teardown the links
	for _, r := range []resource.Resource{dummy0, dummy1, bridge} {
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), r.Metadata()))
	}

	ctest.AssertNoResource[*network.LinkStatus](suite, dummy0Name)
	ctest.AssertNoResource[*network.LinkStatus](suite, dummy1Name)
	ctest.AssertNoResource[*network.LinkStatus](suite, bridgeName)
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestVRF() {
	vrfName := suite.uniqueDummyInterface()
	vrf := network.NewLinkSpec(network.NamespaceName, vrfName)
	*vrf.TypedSpec() = network.LinkSpecSpec{
		Name:    vrfName,
		Type:    nethelpers.LinkEther,
		Kind:    network.LinkKindVRF,
		Up:      true,
		Logical: true,
		VRFMaster: network.VRFMasterSpec{
			Table: 123,
		},
		ConfigLayer: network.ConfigDefault,
	}

	dummy0Name := suite.uniqueDummyInterface()
	dummy0 := network.NewLinkSpec(network.NamespaceName, dummy0Name)
	*dummy0.TypedSpec() = network.LinkSpecSpec{
		Name:    dummy0Name,
		Type:    nethelpers.LinkEther,
		Kind:    "dummy",
		Up:      true,
		Logical: true,
		VRFSlave: network.VRFSlave{
			MasterName: vrfName,
		},
		ConfigLayer: network.ConfigDefault,
	}

	dummy1Name := suite.uniqueDummyInterface()
	dummy1 := network.NewLinkSpec(network.NamespaceName, dummy1Name)
	*dummy1.TypedSpec() = network.LinkSpecSpec{
		Name:    dummy1Name,
		Type:    nethelpers.LinkEther,
		Kind:    "dummy",
		Up:      true,
		Logical: true,
		VRFSlave: network.VRFSlave{
			MasterName: vrfName,
		},
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{dummy0, dummy1, vrf} {
		suite.Create(res)
	}

	ctest.AssertResources(suite, []string{dummy0Name, dummy1Name, vrfName}, func(r *network.LinkStatus, asrt *assert.Assertions) {
		switch r.Metadata().ID() {
		case vrfName:
			asrt.Equal(network.LinkKindVRF, r.TypedSpec().Kind)
			asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
		case dummy0Name, dummy1Name:
			asrt.Equal("dummy", r.TypedSpec().Kind)
			asrt.Equal(nethelpers.OperStateUnknown, r.TypedSpec().OperationalState)
			asrt.NotZero(r.TypedSpec().MasterIndex)
		}
	})

	// attempt to change the vrf table
	ctest.UpdateWithConflicts(suite, vrf, func(r *network.LinkSpec) error {
		r.TypedSpec().VRFMaster.Table = nethelpers.Table124

		return nil
	})

	ctest.AssertResource(suite, vrfName, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(network.LinkKindVRF, r.TypedSpec().Kind)
		asrt.Equal(nethelpers.Table124, r.TypedSpec().VRFMaster.Table)
	})

	// unslave one of the interfaces
	ctest.UpdateWithConflicts(suite, dummy0, func(r *network.LinkSpec) error {
		r.TypedSpec().VRFSlave.MasterName = ""

		return nil
	})

	ctest.AssertResource(suite, dummy0Name, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Zero(r.TypedSpec().MasterIndex)
	})

	// teardown the links
	for _, r := range []resource.Resource{dummy0, dummy1, vrf} {
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), r.Metadata()))
	}

	ctest.AssertNoResource[*network.LinkStatus](suite, dummy0Name)
	ctest.AssertNoResource[*network.LinkStatus](suite, dummy1Name)
	ctest.AssertNoResource[*network.LinkStatus](suite, vrfName)
}

//nolint:gocyclo
func (suite *LinkSpecSuite) TestWireguard() {
	if fipsmode.Strict() {
		suite.T().Skip("skipping test in strict FIPS mode")
	}

	priv, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	pub1, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	pub2, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	wgInterface := suite.uniqueDummyInterface()

	wg := network.NewLinkSpec(network.NamespaceName, wgInterface)
	*wg.TypedSpec() = network.LinkSpecSpec{
		Name:    wgInterface,
		Type:    nethelpers.LinkNone,
		Kind:    "wireguard",
		Up:      true,
		Logical: true,
		Wireguard: network.WireguardSpec{
			PrivateKey:   priv.String(),
			FirewallMark: 1,
			Peers: []network.WireguardPeer{
				{
					PublicKey: pub1.PublicKey().String(),
					Endpoint:  "10.2.0.3:20000",
					AllowedIPs: []netip.Prefix{
						netip.MustParsePrefix("172.24.0.0/16"),
					},
				},
				{
					PublicKey: pub2.PublicKey().String(),
					AllowedIPs: []netip.Prefix{
						netip.MustParsePrefix("172.25.0.0/24"),
					},
				},
			},
		},
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{wg} {
		suite.Create(res)
	}

	ctest.AssertResource(suite, wgInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal("wireguard", r.TypedSpec().Kind)
		asrt.Contains([]nethelpers.OperationalState{nethelpers.OperStateUp, nethelpers.OperStateUnknown}, r.TypedSpec().OperationalState)
		asrt.Equal(priv.PublicKey().String(), r.TypedSpec().Wireguard.PublicKey)
		asrt.Len(r.TypedSpec().Wireguard.Peers, 2)
	})

	// attempt to change wireguard private key
	priv2, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	ctest.UpdateWithConflicts(suite, wg, func(r *network.LinkSpec) error {
		r.TypedSpec().Wireguard.PrivateKey = priv2.String()

		return nil
	})

	ctest.AssertResource(suite, wgInterface, func(r *network.LinkStatus, asrt *assert.Assertions) {
		asrt.Equal(priv2.PublicKey().String(), r.TypedSpec().Wireguard.PublicKey)
	})

	// teardown the links
	for _, r := range []resource.Resource{wg} {
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), r.Metadata()))
	}

	ctest.AssertNoResource[*network.LinkStatus](suite, wgInterface)
}

func TestLinkSpecSuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, &LinkSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				// create fake device ready status
				deviceStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
				deviceStatus.TypedSpec().Ready = true
				suite.Create(deviceStatus)

				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkSpecController{}))
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkStatusController{}))
			},
		},
	})
}

func TestSortBonds(t *testing.T) {
	expected := toResources([]network.LinkSpecSpec{
		{
			Name: "A",
		}, {
			Name: "G",
			BondSlave: network.BondSlave{
				MasterName: "A",
				SlaveIndex: 0,
			},
		}, {
			Name: "C",
		}, {
			Name: "E",
			BondSlave: network.BondSlave{
				MasterName: "C",
				SlaveIndex: 0,
			},
		}, {
			Name: "F",
			BondSlave: network.BondSlave{
				MasterName: "C",
				SlaveIndex: 1,
			},
		}, {
			Name: "B",
			BondSlave: network.BondSlave{
				MasterName: "C",
				SlaveIndex: 2,
			},
		},
	})

	seed := time.Now().Unix()

	rnd := rand.New(rand.NewPCG(uint64(time.Now().Unix()), uint64(time.Now().Unix())))

	for i := range 100 {
		res := safe.NewList[*network.LinkSpec](resource.List{
			Items: safe.ToSlice(expected, func(r *network.LinkSpec) resource.Resource { return r }),
		})

		rnd.Shuffle(res.Len(), res.Swap)
		netctrl.SortBonds(&res)
		require.Equal(t, expected, res, "failed with seed %d iteration %d", seed, i)
	}
}

func toResources(slice []network.LinkSpecSpec) safe.List[*network.LinkSpec] {
	return safe.NewList[*network.LinkSpec](resource.List{
		Items: xslices.Map(slice, func(spec network.LinkSpecSpec) resource.Resource {
			link := network.NewLinkSpec(network.NamespaceName, "bar")
			*link.TypedSpec() = spec

			return link
		}),
	})
}
