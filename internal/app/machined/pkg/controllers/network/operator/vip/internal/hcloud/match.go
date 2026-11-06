// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud

import (
	"net/netip"
	"strings"
)

// FindServerByAliasIP returns the ID of the server which has the alias IP in the network, or 0 if not found.
func FindServerByAliasIP(servers []Server, networkID int64, ip netip.Addr) int64 {
	for _, server := range servers {
		for _, privateNet := range server.PrivateNet {
			if privateNet.Network != networkID {
				continue
			}

			for _, alias := range privateNet.AliasIPs {
				if addr, err := netip.ParseAddr(alias); err == nil && addr == ip {
					return server.ID
				}
			}
		}
	}

	return 0
}

// FindFloatingIP returns the floating IP matching the IP, or nil if not found.
//
// IPv6 floating IPs are networks, they match by the network address (as written in the API response).
func FindFloatingIP(floatingIPs []FloatingIP, ip netip.Addr) *FloatingIP {
	for i := range floatingIPs {
		if addr, ok := floatingIPs[i].Addr(); ok && addr == ip {
			return &floatingIPs[i]
		}
	}

	return nil
}

// Addr returns the address of the floating IP (the network address for IPv6 floating IPs).
func (f *FloatingIP) Addr() (netip.Addr, bool) {
	if strings.Contains(f.IP, "/") {
		prefix, err := netip.ParsePrefix(f.IP)
		if err != nil {
			return netip.Addr{}, false
		}

		return prefix.Addr(), true
	}

	addr, err := netip.ParseAddr(f.IP)

	return addr, err == nil
}

// Contains returns true if the IP is within the network IP range.
func (n *Network) Contains(ip netip.Addr) bool {
	prefix, err := netip.ParsePrefix(n.IPRange)

	return err == nil && prefix.Contains(ip)
}
