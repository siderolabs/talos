// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package addressutil contains helpers working with addresses.
package addressutil

import "net/netip"

// DeduplicateIPPrefixes removes duplicates from the given list of prefixes.
//
// The input list must be sorted.
// DeduplicateIPPrefixes performs in-place deduplication.
func DeduplicateIPPrefixes(in []netip.Prefix) []netip.Prefix {
	// assumes that current is sorted
	n := 0

	var prev netip.Prefix

	for _, x := range in {
		if prev != x {
			in[n] = x
			n++
		}

		prev = x
	}

	return in[:n]
}

// FilterIPs filters the given list of IP prefixes based on the given include and exclude subnets.
//
// If includeSubnets is not empty, only IPs that are in one of the subnets are included.
// If excludeSubnets is not empty, IPs that are in one of the subnets are excluded.
func FilterIPs(addrs []netip.Prefix, includeSubnets, excludeSubnets []netip.Prefix) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(addrs))

outer:
	for _, ip := range addrs {
		if len(includeSubnets) > 0 {
			matchesAny := false

			for _, subnet := range includeSubnets {
				if subnet.Contains(ip.Addr()) {
					matchesAny = true

					break
				}
			}

			if !matchesAny {
				continue outer
			}
		}

		for _, subnet := range excludeSubnets {
			if subnet.Contains(ip.Addr()) {
				continue outer
			}
		}

		result = append(result, ip)
	}

	return result
}
