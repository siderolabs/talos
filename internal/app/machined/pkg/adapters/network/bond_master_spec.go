// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"net/netip"

	"github.com/mdlayher/netlink"
	"github.com/siderolabs/go-pointer"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// BondMasterSpec adapter provides encoding/decoding to netlink structures.
//
//nolint:revive,golint
func BondMasterSpec(r *network.BondMasterSpec) bondMaster {
	return bondMaster{
		BondMasterSpec: r,
	}
}

type bondMaster struct {
	*network.BondMasterSpec
}

// FillDefaults fills zero values with proper default values.
//
//nolint:gocyclo
func (a bondMaster) FillDefaults() {
	bond := a.BondMasterSpec

	if bond.ResendIGMP == 0 {
		bond.ResendIGMP = 1
	}

	if bond.LPInterval == 0 {
		bond.LPInterval = 1
	}

	if bond.PacketsPerSlave == 0 {
		bond.PacketsPerSlave = 1
	}

	if bond.NumPeerNotif == 0 {
		bond.NumPeerNotif = 1
	}

	if bond.Mode != nethelpers.BondModeALB && bond.Mode != nethelpers.BondModeTLB {
		bond.TLBDynamicLB = 1
	}

	if bond.Mode == nethelpers.BondMode8023AD && bond.ADActorSysPrio == 0 {
		bond.ADActorSysPrio = 65535
	}

	if bond.MissedMax == 0 {
		bond.MissedMax = 2
	}

	if bond.Mode != nethelpers.BondMode8023AD {
		bond.ADLACPActive = nethelpers.ADLACPActiveOn
	}
}

// Encode the BondMasterSpec into netlink attributes.
//
//nolint:gocyclo,cyclop
func (a bondMaster) Encode() ([]byte, error) {
	bond := a.BondMasterSpec

	encoder := netlink.NewAttributeEncoder()

	encoder.Uint8(unix.IFLA_BOND_MODE, uint8(bond.Mode))
	encoder.Uint8(unix.IFLA_BOND_XMIT_HASH_POLICY, uint8(bond.HashPolicy))

	if bond.Mode == nethelpers.BondMode8023AD {
		encoder.Uint8(unix.IFLA_BOND_AD_LACP_RATE, uint8(bond.LACPRate))
		encoder.Uint8(unix.IFLA_BOND_AD_LACP_ACTIVE, uint8(bond.ADLACPActive))
	}

	if bond.Mode != nethelpers.BondMode8023AD && bond.Mode != nethelpers.BondModeALB && bond.Mode != nethelpers.BondModeTLB {
		encoder.Uint32(unix.IFLA_BOND_ARP_VALIDATE, uint32(bond.ARPValidate))
	}

	encoder.Uint32(unix.IFLA_BOND_ARP_ALL_TARGETS, uint32(bond.ARPAllTargets))

	if bond.Mode == nethelpers.BondModeActiveBackup || bond.Mode == nethelpers.BondModeALB || bond.Mode == nethelpers.BondModeTLB {
		if bond.PrimaryIndex != nil {
			encoder.Uint32(unix.IFLA_BOND_PRIMARY, *bond.PrimaryIndex)
		}
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

		encoder.Nested(unix.IFLA_BOND_ARP_IP_TARGET, func(nae *netlink.AttributeEncoder) error {
			for i, addr := range bond.ARPIPTargets {
				if !addr.Is4() {
					return fmt.Errorf("%s is not IPV4 address", addr)
				}

				ip := addr.As4()
				nae.Bytes(uint16(i), ip[:])
			}

			return nil
		})

		encoder.Nested(unix.IFLA_BOND_NS_IP6_TARGET, func(nae *netlink.AttributeEncoder) error {
			for i, addr := range bond.NSIP6Targets {
				if !addr.Is6() {
					return fmt.Errorf("%s is not IPV6 address", addr)
				}

				ip := addr.As16()
				nae.Bytes(uint16(i), ip[:])
			}

			return nil
		})
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

	if bond.MissedMax != 0 {
		encoder.Uint8(unix.IFLA_BOND_MISSED_MAX, bond.MissedMax)
	}

	return encoder.Encode()
}

// Decode the BondMasterSpec from netlink attributes.
//
//nolint:gocyclo,cyclop
func (a bondMaster) Decode(data []byte) error {
	bond := a.BondMasterSpec

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
			bond.PrimaryIndex = pointer.To(decoder.Uint32())
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
		case unix.IFLA_BOND_ARP_IP_TARGET:
			decoder.Nested(func(nad *netlink.AttributeDecoder) error {
				for nad.Next() {
					addr, ok := netip.AddrFromSlice(nad.Bytes())

					if ok {
						bond.ARPIPTargets = append(bond.ARPIPTargets, addr)
					} else {
						return fmt.Errorf("invalid ARP IP target")
					}
				}

				return nil
			})
		case unix.IFLA_BOND_NS_IP6_TARGET:
			decoder.Nested(func(nad *netlink.AttributeDecoder) error {
				for nad.Next() {
					addr, ok := netip.AddrFromSlice(nad.Bytes())

					if ok {
						bond.NSIP6Targets = append(bond.NSIP6Targets, addr)
					} else {
						return fmt.Errorf("invalid NS IP6 target")
					}
				}

				return nil
			})
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
		case unix.IFLA_BOND_AD_LACP_ACTIVE:
			bond.ADLACPActive = nethelpers.ADLACPActive(decoder.Uint8())
		case unix.IFLA_BOND_MISSED_MAX:
			bond.MissedMax = decoder.Uint8()
		}
	}

	return decoder.Err()
}
