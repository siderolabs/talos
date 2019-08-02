/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package nic

import (
	"errors"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
)

// Option is the functional option func.
type Option func(*NetworkInterface) error

// defaultOptions defines our default network interface configuration.
func defaultOptions() *NetworkInterface {
	return &NetworkInterface{
		Type:          Single,
		MTU:           1500,
		AddressMethod: []address.Addressing{},
	}
}

// WithName sets the name of the interface to the given name.
func WithName(o string) Option {
	return func(n *NetworkInterface) (err error) {
		n.Name = o
		return err
	}
}

// WithIndex sets the interface index
func WithIndex(o uint32) Option {
	return func(n *NetworkInterface) (err error) {
		n.Index = o
		return err
	}
}

// WithType defines how the interface should be configured - bonded or single.
func WithType(o int) Option {
	return func(n *NetworkInterface) (err error) {
		switch o {
		case Bond:
			n.Type = Bond
		case Single:
			n.Type = Single
		default:
			return errors.New("unsupported network interface type")
		}
		return err
	}
}

// WithMTU defines the MTU for the interface
// TODO: I think we should drop this since MTU is getting set
// by address configuration method ( either via dhcp or userdata )
func WithMTU(mtu uint32) Option {
	return func(n *NetworkInterface) (err error) {
		if (mtu < MinimumMTU) || (mtu > MaximumMTU) {
			return errors.New("mtu is out of acceptable range")
		}

		n.MTU = mtu
		return err
	}
}

// WithSubInterface defines which interfaces make up the bond
func WithSubInterface(o string) Option {
	return func(n *NetworkInterface) (err error) {
		n.SubInterfaces = append(n.SubInterfaces, o)
		return err
	}
}

// WithAddressing defines how the addressing for a given interface
// should be configured
func WithAddressing(a address.Addressing) Option {
	return func(n *NetworkInterface) (err error) {
		n.AddressMethod = append(n.AddressMethod, a)
		return err
	}
}
