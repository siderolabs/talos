// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package address provides utility functions for address parsing.
package address

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// IPPrefixFrom make netip.Prefix from cidr-address and netmask strings.
// address can be IP or CIDR (1.1.1.1 or 1.1.1.1/8 or 1.1.1.1/255.0.0.0)
// netmask can be IP or number (255.255.255.0 or 24 or empty).
func IPPrefixFrom(address, netmask string) (netip.Prefix, error) {
	cidr := strings.SplitN(address, "/", 2)
	if len(cidr) == 1 {
		address = cidr[0]
	} else {
		address = cidr[0]
		netmask = cidr[1]
	}

	ip, err := netip.ParseAddr(address)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("failed to parse ip address: %w", err)
	}

	if netmask == "" {
		if ip.Is4() {
			netmask = "32"
		} else {
			netmask = "128"
		}
	}

	bits, err := strconv.Atoi(netmask)
	if err != nil {
		netmask, err := netip.ParseAddr(netmask)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("failed to parse netmask: %w", err)
		}

		mask, _ := netmask.MarshalBinary() //nolint:errcheck // never fails
		bits, _ = net.IPMask(mask).Size()
	}

	if ip.Is4() && bits > 32 {
		return netip.Prefix{}, errors.New("failed netmask should be the same address family")
	}

	return netip.PrefixFrom(ip, bits), nil
}
