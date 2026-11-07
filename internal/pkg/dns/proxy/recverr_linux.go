// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxy

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// recvErr enables IP_RECVERR on UDP sockets, so that ICMP errors (e.g. port unreachable)
// fail the query immediately instead of waiting for the read timeout.
func recvErr(network, _ string, c syscall.RawConn) error {
	if network != "udp4" && network != "udp6" {
		return nil
	}

	level, opt := unix.SOL_IP, unix.IP_RECVERR
	if network == "udp6" {
		level, opt = unix.SOL_IPV6, unix.IPV6_RECVERR
	}

	return c.Control(func(fd uintptr) {
		// failing to set the option is not fatal: the query would just take longer to fail
		unix.SetsockoptInt(int(fd), level, opt, 1) //nolint:errcheck
	})
}
