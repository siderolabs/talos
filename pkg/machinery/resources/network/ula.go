// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"crypto/sha256"
	"net/netip"
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

	// ULAVirtualSideroLink is the Unique Local Addressing space key for the Virtual SideroLink over GRPC feature.
	ULAVirtualSideroLink = 0x04
)

// ULAPrefix calculates and returns a Talos-specific Unique Local Address prefix for the given purpose.
// This implements a Talos-specific implementation of RFC4193.
// The Talos implementation uses a combination of a 48-bit cluster-unique portion with an 8-bit purpose portion.
func ULAPrefix(clusterID string, purpose ULAPurpose) netip.Prefix {
	var prefixData [16]byte

	hash := sha256.Sum256([]byte(clusterID))

	// Take the last 16 bytes of the clusterID's hash.
	copy(prefixData[:], hash[sha256.Size-16:])

	// Apply the ULA prefix as per RFC4193
	prefixData[0] = 0xfd

	// Apply the Talos-specific ULA Purpose suffix
	prefixData[7] = byte(purpose)

	return netip.PrefixFrom(netip.AddrFrom16(prefixData), 64).Masked()
}

// IsULA checks whether IP address is a Unique Local Address with the specific purpose.
func IsULA(ip netip.Addr, purpose ULAPurpose) bool {
	if !ip.Is6() {
		return false
	}

	raw := ip.As16()

	return raw[0] == 0xfd && raw[7] == byte(purpose)
}

// NotSideroLinkIP is a shorthand for !IsULA(ip, ULASideroLink).
func NotSideroLinkIP(ip netip.Addr) bool {
	return !IsULA(ip, ULASideroLink)
}
