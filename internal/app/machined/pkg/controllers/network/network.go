// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
package network

import (
	"net"

	"github.com/siderolabs/gen/pair/ordered"
	"github.com/siderolabs/go-pointer"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// SetBondSlave sets the bond slave spec.
func SetBondSlave(link *network.LinkSpecSpec, bond ordered.Pair[string, int]) {
	link.BondSlave = network.BondSlave{
		MasterName: bond.F1,
		SlaveIndex: bond.F2,
	}
}

// SendBondMaster sets the bond master spec.
func SendBondMaster(link *network.LinkSpecSpec, bond talosconfig.NetworkBondConfig) {
	link.Logical = true
	link.Kind = network.LinkKindBond
	link.Type = nethelpers.LinkEther
	link.BondMaster.Mode = bond.Mode()
	link.BondMaster.MIIMon = bond.MIIMon().ValueOrZero()
	link.BondMaster.UpDelay = bond.UpDelay().ValueOrZero()
	link.BondMaster.DownDelay = bond.DownDelay().ValueOrZero()
	link.BondMaster.HashPolicy = bond.XmitHashPolicy().ValueOrZero()
	link.BondMaster.ARPInterval = bond.ARPInterval().ValueOrZero()
	link.BondMaster.ARPIPTargets = bond.ARPIPTargets()
	link.BondMaster.NSIP6Targets = bond.NSIP6Targets()
	link.BondMaster.ARPValidate = bond.ARPValidate().ValueOrZero()
	link.BondMaster.ARPAllTargets = bond.ARPAllTargets().ValueOrZero()
	link.BondMaster.LACPRate = bond.LACPRate().ValueOrZero()
	link.BondMaster.FailOverMac = bond.FailOverMAC().ValueOrZero()
	link.BondMaster.ADSelect = bond.ADSelect().ValueOrZero()
	link.BondMaster.ADActorSysPrio = bond.ADActorSysPrio().ValueOrZero()
	link.BondMaster.ADUserPortKey = bond.ADUserPortKey().ValueOrZero()
	link.BondMaster.ADLACPActive = bond.ADLACPActive().ValueOr(nethelpers.ADLACPActiveOn)
	link.BondMaster.PrimaryReselect = bond.PrimaryReselect().ValueOrZero()
	link.BondMaster.ResendIGMP = bond.ResendIGMP().ValueOrZero()
	link.BondMaster.MinLinks = bond.MinLinks().ValueOrZero()
	link.BondMaster.LPInterval = bond.LPInterval().ValueOrZero()
	link.BondMaster.PacketsPerSlave = bond.PacketsPerSlave().ValueOrZero()
	link.BondMaster.NumPeerNotif = bond.NumPeerNotif().ValueOrZero()
	link.BondMaster.TLBDynamicLB = bond.TLBDynamicLB().ValueOrZero()
	link.BondMaster.AllSlavesActive = bond.AllSlavesActive().ValueOrZero()
	link.BondMaster.PeerNotifyDelay = bond.PeerNotifyDelay().ValueOrZero()
	link.BondMaster.MissedMax = bond.MissedMax().ValueOrZero()

	networkadapter.BondMasterSpec(&link.BondMaster).FillDefaults()
}

// SetBondMasterLegacy sets the bond master spec.
//
//nolint:gocyclo
func SetBondMasterLegacy(link *network.LinkSpecSpec, bond talosconfig.Bond) error {
	link.Logical = true
	link.Kind = network.LinkKindBond
	link.Type = nethelpers.LinkEther

	bondMode, err := nethelpers.BondModeByName(bond.Mode())
	if err != nil {
		return err
	}

	hashPolicy, err := nethelpers.BondXmitHashPolicyByName(bond.HashPolicy())
	if err != nil {
		return err
	}

	lacpRate, err := nethelpers.LACPRateByName(bond.LACPRate())
	if err != nil {
		return err
	}

	arpValidate, err := nethelpers.ARPValidateByName(bond.ARPValidate())
	if err != nil {
		return err
	}

	arpAllTargets, err := nethelpers.ARPAllTargetsByName(bond.ARPAllTargets())
	if err != nil {
		return err
	}

	var primary uint32

	if bond.Primary() != "" {
		var iface *net.Interface

		iface, err = net.InterfaceByName(bond.Primary())
		if err != nil {
			return err
		}

		primary = uint32(iface.Index)
	}

	primaryReselect, err := nethelpers.PrimaryReselectByName(bond.PrimaryReselect())
	if err != nil {
		return err
	}

	failOverMAC, err := nethelpers.FailOverMACByName(bond.FailOverMac())
	if err != nil {
		return err
	}

	adSelect, err := nethelpers.ADSelectByName(bond.ADSelect())
	if err != nil {
		return err
	}

	link.BondMaster = network.BondMasterSpec{
		Mode:            bondMode,
		HashPolicy:      hashPolicy,
		LACPRate:        lacpRate,
		ARPValidate:     arpValidate,
		ARPAllTargets:   arpAllTargets,
		PrimaryIndex:    pointer.To(primary),
		PrimaryReselect: primaryReselect,
		FailOverMac:     failOverMAC,
		ADSelect:        adSelect,
		MIIMon:          bond.MIIMon(),
		UpDelay:         bond.UpDelay(),
		DownDelay:       bond.DownDelay(),
		ARPInterval:     bond.ARPInterval(),
		ResendIGMP:      bond.ResendIGMP(),
		MinLinks:        bond.MinLinks(),
		LPInterval:      bond.LPInterval(),
		PacketsPerSlave: bond.PacketsPerSlave(),
		NumPeerNotif:    bond.NumPeerNotif(),
		TLBDynamicLB:    bond.TLBDynamicLB(),
		AllSlavesActive: bond.AllSlavesActive(),
		ADActorSysPrio:  bond.ADActorSysPrio(),
		ADUserPortKey:   bond.ADUserPortKey(),
		PeerNotifyDelay: bond.PeerNotifyDelay(),
		ADLACPActive:    nethelpers.ADLACPActiveOn,
	}
	networkadapter.BondMasterSpec(&link.BondMaster).FillDefaults()

	return nil
}

// SetBridgeSlave sets the bridge slave spec.
func SetBridgeSlave(link *network.LinkSpecSpec, bridge string) {
	link.BridgeSlave = network.BridgeSlave{
		MasterName: bridge,
	}
}

// SetBridgeMasterLegacy sets the bridge master spec.
func SetBridgeMasterLegacy(link *network.LinkSpecSpec, bridge talosconfig.Bridge) error {
	link.Logical = true
	link.Kind = network.LinkKindBridge
	link.Type = nethelpers.LinkEther

	if bridge != nil {
		link.BridgeMaster = network.BridgeMasterSpec{
			STP: network.STPSpec{
				Enabled: bridge.STP().Enabled(),
			},
			VLAN: network.BridgeVLANSpec{
				FilteringEnabled: bridge.VLAN().FilteringEnabled(),
			},
		}
	}

	return nil
}

// SetBridgeMaster sets the bridge master spec.
func SetBridgeMaster(link *network.LinkSpecSpec, bridge talosconfig.NetworkBridgeConfig) {
	link.Logical = true
	link.Kind = network.LinkKindBridge
	link.Type = nethelpers.LinkEther

	link.BridgeMaster = network.BridgeMasterSpec{
		STP: network.STPSpec{
			Enabled: bridge.STP().Enabled().ValueOrZero(),
		},
		VLAN: network.BridgeVLANSpec{
			FilteringEnabled: bridge.VLAN().FilteringEnabled().ValueOrZero(),
		},
	}
}
