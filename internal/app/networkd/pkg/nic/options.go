// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nic

import (
	"github.com/mdlayher/netlink"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
)

// Option is the functional option func.
type Option func(*NetworkInterface) error

// defaultOptions defines our default network interface configuration.
func defaultOptions() *NetworkInterface {
	return &NetworkInterface{
		Bonded:        false,
		MTU:           1500,
		AddressMethod: []address.Addressing{},
		BondSettings:  netlink.NewAttributeEncoder(),
	}
}

// WithDummy indicates that the interface should be a virtual, dummy interface.
func WithDummy() Option {
	return func(n *NetworkInterface) (err error) {
		n.Dummy = true

		return
	}
}

// WithIgnore indicates that the interface should not be processed by talos.
func WithIgnore() Option {
	return func(n *NetworkInterface) (err error) {
		n.Ignore = true

		return
	}
}

// WithName sets the name of the interface to the given name.
func WithName(o string) Option {
	return func(n *NetworkInterface) (err error) {
		n.Name = o

		return err
	}
}

// WithAddressing defines how the addressing for a given interface
// should be configured.
func WithAddressing(a address.Addressing) Option {
	return func(n *NetworkInterface) (err error) {
		n.AddressMethod = append(n.AddressMethod, a)

		return err
	}
}

// WithNoAddressing defines how the addressing for a given interface
// should be configured.
func WithNoAddressing() Option {
	return func(n *NetworkInterface) (err error) {
		n.AddressMethod = []address.Addressing{}

		return err
	}
}
