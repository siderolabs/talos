// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net/netip"

	"go4.org/netipx"
)

// BuildIPSet builds an IPSet from the given include and exclude prefixes.
func BuildIPSet(include, exclude []netip.Prefix) (*netipx.IPSet, error) {
	var builder netipx.IPSetBuilder

	for _, pfx := range include {
		builder.AddPrefix(pfx)
	}

	for _, pfx := range exclude {
		builder.RemovePrefix(pfx)
	}

	return builder.IPSet()
}

// SplitIPSet splits the given IPSet into IPv4 and IPv6 ranges.
func SplitIPSet(set *netipx.IPSet) (ipv4, ipv6 []netipx.IPRange) {
	for _, rng := range set.Ranges() {
		if rng.From().Is4() {
			ipv4 = append(ipv4, rng)
		} else {
			ipv6 = append(ipv6, rng)
		}
	}

	return
}
