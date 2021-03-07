// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Additional information can be found
// https://www.kernel.org/doc/Documentation/networking/bonding.txt.

package nic

import (
	"net"
)

// WithBond defines if the interface should be bonded.
func WithBond(o bool) Option {
	return func(n *NetworkInterface) (err error) {
		n.Bonded = o

		return nil
	}
}

// WithSubInterface defines which interfaces make up the bond.
func WithSubInterface(o ...string) Option {
	return func(n *NetworkInterface) (err error) {
		var found bool

		for _, ifname := range o {
			found = false

			for _, subif := range n.SubInterfaces {
				if ifname == subif.Name {
					found = true

					break
				}
			}

			if found {
				continue
			}

			var iface *net.Interface

			iface, err = net.InterfaceByName(ifname)
			if err != nil {
				return err
			}

			n.SubInterfaces = append(n.SubInterfaces, iface)
		}

		return err
	}
}

// WithBondMode sets the mode the bond should operate in.
func WithBondMode(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var mode BondMode

		if mode, err = BondModeByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_MODE), uint8(mode))

		return err
	}
}

// WithHashPolicy configures the transmit hash policy for the bonded interface.
func WithHashPolicy(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var policy BondXmitHashPolicy

		if policy, err = BondXmitHashPolicyByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_XMIT_HASH_POLICY), uint8(policy))

		return err
	}
}

// WithLACPRate configures the bond LACP rate.
func WithLACPRate(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var rate LACPRate

		if rate, err = LACPRateByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_AD_LACP_RATE), uint8(rate))

		return err
	}
}

// WithUpDelay configures the up delay for interfaces that makes up a bond.
// The value is given in ms.
func WithUpDelay(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_UPDELAY), o)

		return err
	}
}

// WithDownDelay configures the down delay for interfaces that makes up a bond.
// The value is given in ms.
func WithDownDelay(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_DOWNDELAY), o)

		return err
	}
}

// WithMIIMon configures the miimon interval for a bond.
// The value is given in ms.
func WithMIIMon(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_MIIMON), o)

		return err
	}
}

// WithUseCarrier configures how miimon will determine the link status.
func WithUseCarrier(o bool) Option {
	return func(n *NetworkInterface) (err error) {
		// default to 1
		var carrier uint8 = 1

		if !o {
			carrier = 0
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_USE_CARRIER), carrier)

		return err
	}
}

// WithARPInterval specifies the ARP link monitoring frequency in milliseconds.
func WithARPInterval(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_ARP_INTERVAL), o)

		return err
	}
}

// WithARPValidate specifies whether or not ARP probes and replies should be
// validated.
func WithARPValidate(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var valid ARPValidate

		if valid, err = ARPValidateByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint32(uint16(IFLA_BOND_ARP_VALIDATE), uint32(valid))

		return err
	}
}

// WithARPAllTargets specifies the quantity of arp_ip_targets that must be
// reachable in order for the ARP monitor to consider a slave as being up.
func WithARPAllTargets(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var target ARPAllTargets

		if target, err = ARPAllTargetsByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint32(uint16(IFLA_BOND_ARP_ALL_TARGETS), uint32(target))

		return err
	}
}

// WithPrimary specifies which slave is the primary device.
func WithPrimary(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var iface *net.Interface

		if iface, err = net.InterfaceByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_PRIMARY_RESELECT), uint8(iface.Index))

		return err
	}
}

// WithPrimaryReselect specifies the reselection policy for the primary slave.
func WithPrimaryReselect(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var primary PrimaryReselect

		if primary, err = PrimaryReselectByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_PRIMARY_RESELECT), uint8(primary))

		return err
	}
}

// WithFailOverMAC specifies whether active-backup mode should set all
// slaves to the same MAC address.
func WithFailOverMAC(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var fo FailOverMAC

		if fo, err = FailOverMACByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_FAIL_OVER_MAC), uint8(fo))

		return err
	}
}

// WithResendIGMP specifies the number of IGMP membership reports to be issued
// after a failover event.
func WithResendIGMP(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_RESEND_IGMP), o)

		return err
	}
}

// WithNumPeerNotif specifies the number of peer notifications (gratuitous ARPs and
// unsolicited IPv6 Neighbor Advertisements) to be issued after a failover event.
func WithNumPeerNotif(o uint8) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint8(uint16(IFLA_BOND_NUM_PEER_NOTIF), o)

		return err
	}
}

// WithAllSlavesActive specifies that duplicate frames (received on inactive
// ports) should be dropped (0) or delivered (1).
func WithAllSlavesActive(o uint8) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint8(uint16(IFLA_BOND_ALL_SLAVES_ACTIVE), o)

		return err
	}
}

// WithMinLinks specifies the minimum number of links that must be active
// before asserting carrier.
func WithMinLinks(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_MIN_LINKS), o)

		return err
	}
}

// WithLPInterval specifies the number of seconds between instances where
// the bonding driver sends learning packets to each slaves peer switch.
func WithLPInterval(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_LP_INTERVAL), o)

		return err
	}
}

// WithPacketsPerSlave specify the number of packets to transmit through
// a slave before moving to the next one.
func WithPacketsPerSlave(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_PACKETS_PER_SLAVE), o)

		return err
	}
}

// WithADSelect specifies the 802.3ad aggregation selection logic to use.
func WithADSelect(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var sel ADSelect

		if sel, err = ADSelectByName(o); err != nil {
			return err
		}

		n.BondSettings.Uint8(uint16(IFLA_BOND_AD_SELECT), uint8(sel))

		return err
	}
}

// WithADActorSysPrio in an AD system, this specifies the system priority.
func WithADActorSysPrio(o uint16) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint16(uint16(IFLA_BOND_AD_ACTOR_SYS_PRIO), o)

		return err
	}
}

// WithADUserPortKey specifies the upper 10 bits of the port key.
func WithADUserPortKey(o uint16) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint16(uint16(IFLA_BOND_AD_USER_PORT_KEY), o)

		return err
	}
}

// WithTLBDynamicLB specifies if dynamic shuffling of flows is enabled in
// tlb mode.
func WithTLBDynamicLB(o uint8) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint8(uint16(IFLA_BOND_TLB_DYNAMIC_LB), o)

		return err
	}
}

// WithPeerNotifyDelay specifies the delay between each peer notification
// (gratuitous ARP and unsolicited IPv6 Neighbor Advertisement) when they
// are issued after a failover event.
// The value is given in ms.
func WithPeerNotifyDelay(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Uint32(uint16(IFLA_BOND_PEER_NOTIF_DELAY), o)

		return err
	}
}

/*
// o = mac addr
// [IFLA_BOND_AD_ACTOR_SYSTEM]	= { .type = NLA_BINARY, .len  = ETH_ALEN },
func WithADActorSystem(o string) Option {
	return func(n *NetworkInterface) (err error) {
		return err
	}
}

// Not sure this is the right way to do nested attributes
func WithARPIPTarget(o []string) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings.Nested(uint16(IFLA_BOND_ARP_IP_TARGET), netlink.NewAttributeEncoder().String(uint16(IFLA_BOND_ARP_IP_TARGET), strings.Join(",", o)))
		return err
	}
}

// [IFLA_BOND_AD_INFO]		= { .type = NLA_NESTED },
// func WithInfo(o []string) Option {

// Not adding Active Slave since this is more of a
// runtime adjustment versus config
// [IFLA_BOND_ACTIVE_SLAVE]	= { .type = NLA_U32 },
*/
