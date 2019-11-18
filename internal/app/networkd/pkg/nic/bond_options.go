// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic

import (
	"errors"

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
		n.SubInterfaces = append(n.SubInterfaces, o...)
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
			// ? need a bettwe way to identify length
			Length: 8,
			Type:   uint16(IFLA_BOND_MIIMON),
			Data:   []byte{byte(o)},
		},
		)
		return err
	}
}
