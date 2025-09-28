// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package addressutil

import (
	"net"
	"net/netip"

	"go4.org/netipx"
)

// BroadcastAddr calculates the broadcast address for the given IPv4 prefix.
//
// If the address is not IPv4 or the prefix length is 31 or 32, nil is returned.
func BroadcastAddr(addr netip.Prefix) net.IP {
	if !addr.Addr().Is4() {
		return nil
	}

	if addr.Bits() >= 31 {
		return nil
	}

	ipnet := netipx.PrefixIPNet(addr)

	ip := ipnet.IP.To4()
	if ip == nil {
		return nil
	}

	mask := net.IP(ipnet.Mask).To4()

	n := len(ip)
	if n != len(mask) {
		return nil
	}

	out := make(net.IP, n)

	for i := range n {
		out[i] = ip[i] | ^mask[i]
	}

	return out
}
