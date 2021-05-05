// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import "inet.af/netaddr"

func mergeIPPortSets(a []netaddr.IPPort, b []netaddr.IPPort) (out []netaddr.IPPort, changed bool) {
	var found bool

	var existing netaddr.IPPort

	out = a

	for _, ip := range b {
		found = false

		for _, existing = range out {
			if ip == existing {
				found = true

				break
			}
		}

		if !found {
			changed = true

			out = append(out, ip)
		}
	}

	return out, changed
}

func mergeIPSets(a []netaddr.IP, b []netaddr.IP) (out []netaddr.IP, changed bool) {
	var found bool

	var existing netaddr.IP

	out = a

	for _, ip := range b {
		found = false

		for _, existing = range out {
			if ip == existing {
				found = true

				break
			}
		}

		if !found {
			changed = true

			out = append(out, ip)
		}
	}

	return out, changed
}
