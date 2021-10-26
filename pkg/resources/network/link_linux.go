// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"encoding/binary"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// Encode the VLANSpec into netlink attributes.
func (vlan *VLANSpec) Encode() ([]byte, error) {
	encoder := netlink.NewAttributeEncoder()

	encoder.Uint16(unix.IFLA_VLAN_ID, vlan.VID)

	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(vlan.Protocol))
	encoder.Bytes(unix.IFLA_VLAN_PROTOCOL, buf)

	return encoder.Encode()
}

// Decode the VLANSpec from netlink attributes.
func (vlan *VLANSpec) Decode(data []byte) error {
	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		switch decoder.Type() {
		case unix.IFLA_VLAN_ID:
			vlan.VID = decoder.Uint16()
		case unix.IFLA_VLAN_PROTOCOL:
			vlan.Protocol = nethelpers.VLANProtocol(binary.BigEndian.Uint16(decoder.Bytes()))
		}
	}

	return decoder.Err()
}

// Encode the BondMasterSpec into netlink attributes.
//
//nolint:gocyclo
func (bond *BondMasterSpec) Encode() ([]byte, error) {
	encoder := netlink.NewAttributeEncoder()

	encoder.Uint8(unix.IFLA_BOND_MODE, uint8(bond.Mode))
	encoder.Uint8(unix.IFLA_BOND_XMIT_HASH_POLICY, uint8(bond.HashPolicy))

	if bond.Mode == nethelpers.BondMode8023AD {
		encoder.Uint8(unix.IFLA_BOND_AD_LACP_RATE, uint8(bond.LACPRate))
	}

	if bond.Mode != nethelpers.BondMode8023AD && bond.Mode != nethelpers.BondModeALB && bond.Mode != nethelpers.BondModeTLB {
		encoder.Uint32(unix.IFLA_BOND_ARP_VALIDATE, uint32(bond.ARPValidate))
	}

	encoder.Uint32(unix.IFLA_BOND_ARP_ALL_TARGETS, uint32(bond.ARPAllTargets))

	if bond.Mode == nethelpers.BondModeActiveBackup || bond.Mode == nethelpers.BondModeALB || bond.Mode == nethelpers.BondModeTLB {
		encoder.Uint32(unix.IFLA_BOND_PRIMARY, bond.PrimaryIndex)
	}

	encoder.Uint8(unix.IFLA_BOND_PRIMARY_RESELECT, uint8(bond.PrimaryReselect))
	encoder.Uint8(unix.IFLA_BOND_FAIL_OVER_MAC, uint8(bond.FailOverMac))
	encoder.Uint8(unix.IFLA_BOND_AD_SELECT, uint8(bond.ADSelect))
	encoder.Uint32(unix.IFLA_BOND_MIIMON, bond.MIIMon)

	if bond.MIIMon != 0 {
		encoder.Uint32(unix.IFLA_BOND_UPDELAY, bond.UpDelay)
		encoder.Uint32(unix.IFLA_BOND_DOWNDELAY, bond.DownDelay)
	}

	if bond.Mode != nethelpers.BondMode8023AD && bond.Mode != nethelpers.BondModeALB && bond.Mode != nethelpers.BondModeTLB {
		encoder.Uint32(unix.IFLA_BOND_ARP_INTERVAL, bond.ARPInterval)
	}

	encoder.Uint32(unix.IFLA_BOND_RESEND_IGMP, bond.ResendIGMP)
	encoder.Uint32(unix.IFLA_BOND_MIN_LINKS, bond.MinLinks)
	encoder.Uint32(unix.IFLA_BOND_LP_INTERVAL, bond.LPInterval)

	if bond.Mode == nethelpers.BondModeRoundrobin {
		encoder.Uint32(unix.IFLA_BOND_PACKETS_PER_SLAVE, bond.PacketsPerSlave)
	}

	encoder.Uint8(unix.IFLA_BOND_NUM_PEER_NOTIF, bond.NumPeerNotif)

	if bond.Mode == nethelpers.BondModeALB || bond.Mode == nethelpers.BondModeTLB {
		encoder.Uint8(unix.IFLA_BOND_TLB_DYNAMIC_LB, bond.TLBDynamicLB)
	}

	encoder.Uint8(unix.IFLA_BOND_ALL_SLAVES_ACTIVE, bond.AllSlavesActive)

	var useCarrier uint8

	if bond.UseCarrier {
		useCarrier = 1
	}

	encoder.Uint8(unix.IFLA_BOND_USE_CARRIER, useCarrier)

	if bond.Mode == nethelpers.BondMode8023AD {
		encoder.Uint16(unix.IFLA_BOND_AD_ACTOR_SYS_PRIO, bond.ADActorSysPrio)
		encoder.Uint16(unix.IFLA_BOND_AD_USER_PORT_KEY, bond.ADUserPortKey)
	}

	if bond.MIIMon != 0 {
		encoder.Uint32(unix.IFLA_BOND_PEER_NOTIF_DELAY, bond.PeerNotifyDelay)
	}

	return encoder.Encode()
}

// Decode the BondMasterSpec from netlink attributes.
//
//nolint:gocyclo,cyclop
func (bond *BondMasterSpec) Decode(data []byte) error {
	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		switch decoder.Type() {
		case unix.IFLA_BOND_MODE:
			bond.Mode = nethelpers.BondMode(decoder.Uint8())
		case unix.IFLA_BOND_XMIT_HASH_POLICY:
			bond.HashPolicy = nethelpers.BondXmitHashPolicy(decoder.Uint8())
		case unix.IFLA_BOND_AD_LACP_RATE:
			bond.LACPRate = nethelpers.LACPRate(decoder.Uint8())
		case unix.IFLA_BOND_ARP_VALIDATE:
			bond.ARPValidate = nethelpers.ARPValidate(decoder.Uint32())
		case unix.IFLA_BOND_ARP_ALL_TARGETS:
			bond.ARPAllTargets = nethelpers.ARPAllTargets(decoder.Uint32())
		case unix.IFLA_BOND_PRIMARY:
			bond.PrimaryIndex = decoder.Uint32()
		case unix.IFLA_BOND_PRIMARY_RESELECT:
			bond.PrimaryReselect = nethelpers.PrimaryReselect(decoder.Uint8())
		case unix.IFLA_BOND_FAIL_OVER_MAC:
			bond.FailOverMac = nethelpers.FailOverMAC(decoder.Uint8())
		case unix.IFLA_BOND_AD_SELECT:
			bond.ADSelect = nethelpers.ADSelect(decoder.Uint8())
		case unix.IFLA_BOND_MIIMON:
			bond.MIIMon = decoder.Uint32()
		case unix.IFLA_BOND_UPDELAY:
			bond.UpDelay = decoder.Uint32()
		case unix.IFLA_BOND_DOWNDELAY:
			bond.DownDelay = decoder.Uint32()
		case unix.IFLA_BOND_ARP_INTERVAL:
			bond.ARPInterval = decoder.Uint32()
		case unix.IFLA_BOND_RESEND_IGMP:
			bond.ResendIGMP = decoder.Uint32()
		case unix.IFLA_BOND_MIN_LINKS:
			bond.MinLinks = decoder.Uint32()
		case unix.IFLA_BOND_LP_INTERVAL:
			bond.LPInterval = decoder.Uint32()
		case unix.IFLA_BOND_PACKETS_PER_SLAVE:
			bond.PacketsPerSlave = decoder.Uint32()
		case unix.IFLA_BOND_NUM_PEER_NOTIF:
			bond.NumPeerNotif = decoder.Uint8()
		case unix.IFLA_BOND_TLB_DYNAMIC_LB:
			bond.TLBDynamicLB = decoder.Uint8()
		case unix.IFLA_BOND_ALL_SLAVES_ACTIVE:
			bond.AllSlavesActive = decoder.Uint8()
		case unix.IFLA_BOND_USE_CARRIER:
			bond.UseCarrier = decoder.Uint8() == 1
		case unix.IFLA_BOND_AD_ACTOR_SYS_PRIO:
			bond.ADActorSysPrio = decoder.Uint16()
		case unix.IFLA_BOND_AD_USER_PORT_KEY:
			bond.ADUserPortKey = decoder.Uint16()
		case unix.IFLA_BOND_PEER_NOTIF_DELAY:
			bond.PeerNotifyDelay = decoder.Uint32()
		}
	}

	return decoder.Err()
}
