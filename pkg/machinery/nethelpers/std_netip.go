// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"net"
	"net/netip"

	"github.com/siderolabs/gen/slices"
	"go4.org/netipx"
)

// MapStdToNetIP converts a slice of net.IP to a slice of netip.Addr.
func MapStdToNetIP(in []net.IP) []netip.Addr {
	return slices.Map(in, func(std net.IP) netip.Addr {
		addr, _ := netipx.FromStdIP(std)

		return addr.Unmap()
	})
}

// MapNetIPToStd converts a slice of netip.Addr to a slice of net.IP.
func MapNetIPToStd(in []netip.Addr) []net.IP {
	return slices.Map(in, func(addr netip.Addr) net.IP {
		return addr.Unmap().AsSlice()
	})
}
