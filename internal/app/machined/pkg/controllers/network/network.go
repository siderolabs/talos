// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
package network

import (
	"net"

	networkadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/network"
	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/ordered"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// DefaultRouteMetric is the default route metric if no metric was specified explicitly.
const DefaultRouteMetric = 1024

// SetBondSlave sets the bond slave spec.
func SetBondSlave(link *network.LinkSpecSpec, bond ordered.Pair[string, int]) {
	link.BondSlave = network.BondSlave{
		MasterName: bond.F1,
		SlaveIndex: bond.F2,
	}
}

// SetBondMaster sets the bond master spec.
//nolint:gocyclo
func SetBondMaster(link *network.LinkSpecSpec, bond talosconfig.Bond) error {
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
		PrimaryIndex:    primary,
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
		UseCarrier:      bond.UseCarrier(),
		ADActorSysPrio:  bond.ADActorSysPrio(),
		ADUserPortKey:   bond.ADUserPortKey(),
		PeerNotifyDelay: bond.PeerNotifyDelay(),
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

// SetBridgeMaster sets the bridge master spec.
//nolint:gocyclo
func SetBridgeMaster(link *network.LinkSpecSpec, bridge talosconfig.Bridge) error {
	link.Logical = true
	link.Kind = network.LinkKindBridge
	link.Type = nethelpers.LinkEther
	link.BridgeMaster = network.BridgeMasterSpec{STPEnabled: bridge.STPEnabled()}

	return nil
}
