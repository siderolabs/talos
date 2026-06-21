// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"fmt"
	"net/netip"
)

// Prefix is a wrapper for netip.Prefix.
//
// It implements IsZero() so that yaml.Marshal correctly skips empty values.
//
//docgen:nodoc
type Prefix struct {
	netip.Prefix
}

// IsZero implements yaml.IsZeroer interface.
func (n Prefix) IsZero() bool {
	return n.Prefix == netip.Prefix{}
}

// Merge implements the merger interface: the wrapped value is treated as atomic
// (netip types have unexported fields and cannot be deep-merged via reflection).
func (n *Prefix) Merge(other any) error {
	otherPrefix, ok := other.(Prefix)
	if !ok {
		return fmt.Errorf("cannot merge Prefix with %T", other)
	}

	if !otherPrefix.IsZero() {
		*n = otherPrefix
	}

	return nil
}

// Addr is a wrapper for netip.Addr.
//
// It implements IsZero() so that yaml.Marshal correctly skips empty values.
//
//docgen:nodoc
type Addr struct {
	netip.Addr
}

// IsZero implements yaml.IsZeroer interface.
func (n Addr) IsZero() bool {
	return n.Addr == netip.Addr{}
}

// Merge implements the merger interface: the wrapped value is treated as atomic
// (netip types have unexported fields and cannot be deep-merged via reflection).
func (n *Addr) Merge(other any) error {
	otherAddr, ok := other.(Addr)
	if !ok {
		return fmt.Errorf("cannot merge Addr with %T", other)
	}

	if !otherAddr.IsZero() {
		*n = otherAddr
	}

	return nil
}

// AddrPort is a wrapper for netip.AddrPort.
//
// It implements IsZero() so that yaml.Marshal correctly skips empty values.
//
//docgen:nodoc
type AddrPort struct {
	netip.AddrPort
}

// IsZero implements yaml.IsZeroer interface.
func (n AddrPort) IsZero() bool {
	return n.AddrPort == netip.AddrPort{}
}

// Merge implements the merger interface: the wrapped value is treated as atomic
// (netip types have unexported fields and cannot be deep-merged via reflection).
func (n *AddrPort) Merge(other any) error {
	otherAddrPort, ok := other.(AddrPort)
	if !ok {
		return fmt.Errorf("cannot merge AddrPort with %T", other)
	}

	if !otherAddrPort.IsZero() {
		*n = otherAddrPort
	}

	return nil
}
