// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic

import (
	"errors"
	"net"

	"github.com/mdlayher/netlink"
)

// WithBond defines if the interface should be bonded.
func WithBond(o bool) Option {
	return func(n *NetworkInterface) (err error) {
		n.Bonded = o
		return errors.New("unsupported network interface type")
	}
}

// WithSubInterface defines which interfaces make up the bond
func WithSubInterface(o ...string) Option {
	return func(n *NetworkInterface) (err error) {
		for _, ifname := range o {
			iface, err := net.InterfaceByName(ifname)
			if err != nil {
				return err
			}
			n.SubInterfaces = append(n.SubInterfaces, iface)
		}
		return err
	}
}

func WithBondMode(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var mode BondMode
		if mode, err = BondModeByName(o); err != nil {
			return err
		}

		n.BondSettings = append(n.BondSettings, netlink.Attribute{
			Type: uint16(IFLA_BOND_MODE),
			Data: []byte{byte(mode)},
		},
		)
		return err
	}
}

func WithHashPolicy(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var policy BondXmitHashPolicy
		if policy, err = BondXmitHashPolicyByName(o); err != nil {
			return err
		}

		n.BondSettings = append(n.BondSettings, netlink.Attribute{
			Type: uint16(IFLA_BOND_XMIT_HASH_POLICY),
			Data: []byte{byte(policy)},
		},
		)
		return err
	}
}

func WithLACPRate(o string) Option {
	return func(n *NetworkInterface) (err error) {
		var rate LACPRate
		if rate, err = LACPRateByName(o); err != nil {
			return err
		}

		n.BondSettings = append(n.BondSettings, netlink.Attribute{
			Type: uint16(IFLA_BOND_AD_LACP_RATE),
			Data: []byte{byte(rate)},
		},
		)
		return err
	}
}

func WithUpDelay(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings = append(n.BondSettings, netlink.Attribute{
			Length: 8,
			Type:   uint16(IFLA_BOND_UPDELAY),
			Data:   []byte{byte(o)},
		},
		)
		return err
	}
}

func WithDownDelay(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings = append(n.BondSettings, netlink.Attribute{
			Length: 8,
			Type:   uint16(IFLA_BOND_DOWNDELAY),
			Data:   []byte{byte(o)},
		},
		)
		return err
	}
}

func WithMIIMon(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.BondSettings = append(n.BondSettings, netlink.Attribute{
			// TODO check iproute for guidance here
			// ? need a better way to identify length
			// ohhh maybe map[string]len ?
			Length: 8,
			Type:   uint16(IFLA_BOND_MIIMON),
			Data:   []byte{byte(o)},
		},
		)
		return err
	}
}

/*
	Ref: length
	__u8 mode, use_carrier, primary_reselect, fail_over_mac;
	__u8 xmit_hash_policy, num_peer_notif, all_slaves_active;
	__u8 lacp_rate, ad_select, tlb_dynamic_lb;
	__u16 ad_user_port_key, ad_actor_sys_prio;
	__u32 miimon, updelay, downdelay, peer_notify_delay, arp_interval, arp_validate;
	__u32 arp_all_targets, resend_igmp, min_links, lp_interval;
	__u32 packets_per_slave;
*/
