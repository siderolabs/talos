// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import "net/netip"

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
