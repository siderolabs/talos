// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"net"

	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
)

// MapStdToNetAddr converts a slice of net.IP to a slice of netaddr.Addr.
func MapStdToNetAddr(in []net.IP) []netaddr.IP {
	return slices.Map(in, func(std net.IP) netaddr.IP {
		addr, _ := netaddr.FromStdIP(std)

		return addr.Unmap()
	})
}

// MapNetAddrToStd converts a slice of netaddr.Addr to a slice of net.IP.
func MapNetAddrToStd(in []netaddr.IP) []net.IP {
	return slices.Map(in, func(addr netaddr.IP) net.IP {
		return addr.Unmap().IPAddr().IP
	})
}
