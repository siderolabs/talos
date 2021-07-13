// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import (
	"net"

	"inet.af/netaddr"
)

func resolveCanonicalName(name string, port uint16) (out []netaddr.IPPort, err error) {
	ips, err := net.LookupIP(name)
	if err != nil {
		return nil, err
	}

	for _, stdIP := range ips {
		ip, ok := netaddr.FromStdIP(stdIP)
		if ok {
			out = append(out, netaddr.IPPortFrom(ip, port))
		}
	}

	return out, nil
}

func resolveHostname(name string, defaultPort uint16) (out []netaddr.IPPort, err error) {
	// Note: this is a generic SRV lookup on the hostname,
	// not the Wireguard-specific DNS-SRV service discovery system.
	_, records, err := net.LookupSRV("wireguard", "udp", name)
	if err == nil {
		for _, r := range records {
			srvout, err := resolveCanonicalName(r.Target, r.Port)
			if err == nil {
				out = append(out, srvout...)
			}
		}

		// If we had SRV records, they should be used in preference to AAAA or A records.
		if len(records) > 0 {
			return out, nil
		}
	}

	return resolveCanonicalName(name, defaultPort)
}
