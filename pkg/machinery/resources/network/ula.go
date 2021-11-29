// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"crypto/sha256"
	"net"

	"inet.af/netaddr"
)

// ULAPurpose is the Unique Local Addressing key for the Talos-specific purpose of the prefix.
type ULAPurpose byte

const (
	// ULAUnknown indicates an unknown ULA Purpose.
	ULAUnknown = 0x00

	// ULABootstrap is the Unique Local Addressing space key for the Talos Self-Bootstrapping protocol.
	ULABootstrap = 0x01

	// ULAKubeSpan is the Unique Local Addressing space key for the Talos KubeSpan feature.
	ULAKubeSpan = 0x02

	// ULASideroLink is the Unique Local Addressing space key for the SideroLink feature.
	ULASideroLink = 0x03
)

// ULAPrefix calculates and returns a Talos-specific Unique Local Address prefix for the given purpose.
// This implements a Talos-specific implementation of RFC4193.
// The Talos implementation uses a combination of a 48-bit cluster-unique portion with an 8-bit purpose portion.
func ULAPrefix(clusterID string, purpose ULAPurpose) netaddr.IPPrefix {
	var prefixData [16]byte

	hash := sha256.Sum256([]byte(clusterID))

	// Take the last 16 bytes of the clusterID's hash.
	copy(prefixData[:], hash[sha256.Size-16:])

	// Apply the ULA prefix as per RFC4193
	prefixData[0] = 0xfd

	// Apply the Talos-specific ULA Purpose suffix
	prefixData[7] = byte(purpose)

	return netaddr.IPPrefixFrom(netaddr.IPFrom16(prefixData), 64).Masked()
}

// IsULA checks whether IP address is a Unique Local Address with the specific purpose.
func IsULA(ip netaddr.IP, purpose ULAPurpose) bool {
	if !ip.Is6() {
		return false
	}

	raw := ip.As16()

	return raw[0] == 0xfd && raw[7] == byte(purpose)
}

// IsStdULA implements IsULA for stdlib net.IP.
func IsStdULA(ip net.IP, purpose ULAPurpose) bool {
	addr, ok := netaddr.FromStdIP(ip)
	if !ok {
		return false
	}

	return IsULA(addr, purpose)
}

// NotSideroLinkStdIP is a shorthand for !IsStdULA(ip, ULASideroLink).
func NotSideroLinkStdIP(ip net.IP) bool {
	return !IsStdULA(ip, ULASideroLink)
}
