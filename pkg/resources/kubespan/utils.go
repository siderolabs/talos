// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"fmt"
	"net"

	"github.com/mdlayher/netx/eui64"
	"inet.af/netaddr"
)

func wgEUI64(prefix netaddr.IPPrefix, mac net.HardwareAddr) (out netaddr.IPPrefix, err error) {
	if prefix.IsZero() {
		return out, fmt.Errorf("cannot calculate IP from zero prefix")
	}

	stdIP, err := eui64.ParseMAC(prefix.IPNet().IP, mac)
	if err != nil {
		return out, fmt.Errorf("failed to parse MAC into EUI-64 address: %w", err)
	}

	ip, ok := netaddr.FromStdIP(stdIP)
	if !ok {
		return out, fmt.Errorf("failed to parse intermediate standard IP %q: %w", stdIP.String(), err)
	}

	return netaddr.IPPrefixFrom(ip, ip.BitLen()), nil
}
