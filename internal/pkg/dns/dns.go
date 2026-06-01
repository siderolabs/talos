// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dns provides dns server implementation.
package dns

import (
	"context"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// NewTCPListener creates a new TCP listener.
func NewTCPListener(network, addr string, control ControlFn) (net.Listener, error) {
	network, ok := networkNames[network]
	if !ok {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	lc := net.ListenConfig{Control: control}

	return lc.Listen(context.Background(), network, addr)
}

// NewUDPPacketConn creates a new UDP packet connection.
func NewUDPPacketConn(network, addr string, control ControlFn) (net.PacketConn, error) {
	network, ok := networkNames[network]
	if !ok {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	lc := net.ListenConfig{Control: control}

	return lc.ListenPacket(context.Background(), network, addr)
}

// ControlFn is an alias to [net.ListenConfig.Control] function.
type ControlFn = func(string, string, syscall.RawConn) error

// MakeControl creates a control function for setting socket options.
func MakeControl(network string, forwardEnabled bool) (ControlFn, error) {
	maxHops := 1

	if forwardEnabled {
		maxHops = 2
	}

	var options []controlOptions

	switch network {
	case "tcp", "tcp4":
		options = []controlOptions{
			{unix.IPPROTO_IP, unix.IP_RECVTTL, maxHops, "failed to set IP_RECVTTL"},
			{unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 5, "failed to set TCP_FASTOPEN"}, // tcp specific stuff from systemd
			{unix.IPPROTO_TCP, unix.TCP_NODELAY, 1, "failed to set TCP_NODELAY"},   // tcp specific stuff from systemd
			{unix.IPPROTO_IP, unix.IP_TTL, maxHops, "failed to set IP_TTL"},
		}
	case "tcp6":
		options = []controlOptions{
			{unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, maxHops, "failed to set IPV6_RECVHOPLIMIT"},
			{unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 5, "failed to set TCP_FASTOPEN"}, // tcp specific stuff from systemd
			{unix.IPPROTO_TCP, unix.TCP_NODELAY, 1, "failed to set TCP_NODELAY"},   // tcp specific stuff from systemd
			{unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, maxHops, "failed to set IPV6_UNICAST_HOPS"},
		}
	case "udp", "udp4":
		options = []controlOptions{
			{unix.IPPROTO_IP, unix.IP_RECVTTL, maxHops, "failed to set IP_RECVTTL"},
			{unix.IPPROTO_IP, unix.IP_TTL, maxHops, "failed to set IP_TTL"},
		}
	case "udp6":
		options = []controlOptions{
			{unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, maxHops, "failed to set IPV6_RECVHOPLIMIT"},
			{unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, maxHops, "failed to set IPV6_UNICAST_HOPS"},
		}
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	return func(_ string, _ string, c syscall.RawConn) error {
		var resErr error

		err := c.Control(func(fd uintptr) {
			for _, opt := range options {
				opErr := unix.SetsockoptInt(int(fd), opt.level, opt.opt, opt.val)
				if opErr != nil {
					resErr = fmt.Errorf(opt.errorMessage+": %w", opErr)

					return
				}
			}
		})
		if err != nil {
			return fmt.Errorf("failed in control call: %w", err)
		}

		if resErr != nil {
			return fmt.Errorf("failed to set socket options: %w", resErr)
		}

		return nil
	}, nil
}

type controlOptions struct {
	level        int
	opt          int
	val          int
	errorMessage string
}

var networkNames = map[string]string{
	"tcp":  "tcp4",
	"tcp4": "tcp4",
	"tcp6": "tcp6",
	"udp":  "udp4",
	"udp4": "udp4",
	"udp6": "udp6",
}
