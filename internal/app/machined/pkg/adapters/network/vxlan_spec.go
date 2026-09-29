// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"encoding/binary"
	"net/netip"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// VXLANSpec adapter provides encoding/decoding to netlink structures.
//
// parentIndex is the interface index of the physical device used as the tunnel endpoint: unlike
// VLANs and macvlans, the kernel doesn't report the parent of a VXLAN link via IFLA_LINK, it is
// carried inside the link info as IFLA_VXLAN_LINK, so it is encoded and decoded here. It may be nil
// when the caller doesn't care about the parent.
//
// The attributes are encoded/decoded by hand (like the other adapters in this package) on purpose:
// github.com/jsimonetti/rtnetlink/v2/driver registers its drivers in a global rtnetlink registry from
// init(), which makes rtnetlink decode IFLA_INFO_DATA into typed driver structs instead of raw bytes
// for every kind it knows about, breaking rawLinkData for bonds, bridges, VLANs, macvlans and veths.
//
//nolint:revive
func VXLANSpec(r *network.VXLANSpec, parentIndex *uint32) vxlanSpec {
	if parentIndex == nil {
		parentIndex = new(uint32)
	}

	return vxlanSpec{
		VXLANSpec:   r,
		ParentIndex: parentIndex,
	}
}

type vxlanSpec struct {
	*network.VXLANSpec

	ParentIndex *uint32
}

// DefaultVXLANPort is the destination UDP port used when the spec doesn't specify one.
//
// It matches the IANA-assigned VXLAN port, and it is set explicitly on link creation, as the kernel
// default is the (pre-standard) Linux port 8472.
const DefaultVXLANPort = 4789

// NormalizeVXLANSpec brings the spec into the form the kernel reports it back in, so that a desired
// spec can be compared with the one decoded from an existing link:
//
//   - the destination port is always set, as the kernel always reports one;
//   - IPv4-mapped IPv6 addresses are unmapped, as the kernel reports IPv4 addresses as such;
//   - the zone is dropped, as it is not sent to the kernel;
//   - unspecified addresses are zeroed out, as the kernel doesn't report them at all.
func NormalizeVXLANSpec(vxlan network.VXLANSpec) network.VXLANSpec {
	if vxlan.Port == 0 {
		vxlan.Port = DefaultVXLANPort
	}

	vxlan.Local = normalizeVXLANAddr(vxlan.Local)
	vxlan.Group = normalizeVXLANAddr(vxlan.Group)

	return vxlan
}

func normalizeVXLANAddr(addr netip.Addr) netip.Addr {
	addr = addr.Unmap().WithZone("")

	if !addr.IsValid() || addr.IsUnspecified() {
		return netip.Addr{}
	}

	return addr
}

// Encode the VXLANSpec into netlink attributes.
func (a vxlanSpec) Encode() ([]byte, error) {
	vxlan := NormalizeVXLANSpec(*a.VXLANSpec)

	encoder := netlink.NewAttributeEncoder()

	encoder.Uint32(unix.IFLA_VXLAN_ID, vxlan.ID)
	encoder.Uint32(unix.IFLA_VXLAN_LINK, *a.ParentIndex)

	if local := vxlan.Local; local.IsValid() {
		if local.Is4() {
			encoder.Bytes(unix.IFLA_VXLAN_LOCAL, local.AsSlice())
		} else {
			encoder.Bytes(unix.IFLA_VXLAN_LOCAL6, local.AsSlice())
		}
	}

	if group := vxlan.Group; group.IsValid() {
		if group.Is4() {
			encoder.Bytes(unix.IFLA_VXLAN_GROUP, group.AsSlice())
		} else {
			encoder.Bytes(unix.IFLA_VXLAN_GROUP6, group.AsSlice())
		}
	}

	// the destination port is in network byte order
	//
	// it is always encoded: with no IFLA_VXLAN_PORT the kernel falls back to its own default (8472),
	// which wouldn't match the port the spec is compared against
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, vxlan.Port)
	encoder.Bytes(unix.IFLA_VXLAN_PORT, buf)

	learning := uint8(0)
	if vxlan.Learning {
		learning = 1
	}

	encoder.Uint8(unix.IFLA_VXLAN_LEARNING, learning)

	return encoder.Encode()
}

// Decode the VXLANSpec from netlink attributes.
func (a vxlanSpec) Decode(data []byte) error {
	vxlan := a.VXLANSpec

	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		switch decoder.Type() {
		case unix.IFLA_VXLAN_ID:
			vxlan.ID = decoder.Uint32()
		case unix.IFLA_VXLAN_LINK:
			*a.ParentIndex = decoder.Uint32()
		case unix.IFLA_VXLAN_LOCAL, unix.IFLA_VXLAN_LOCAL6:
			vxlan.Local, _ = netip.AddrFromSlice(decoder.Bytes())
		case unix.IFLA_VXLAN_GROUP, unix.IFLA_VXLAN_GROUP6:
			vxlan.Group, _ = netip.AddrFromSlice(decoder.Bytes())
		case unix.IFLA_VXLAN_PORT:
			// the destination port is in network byte order
			if buf := decoder.Bytes(); len(buf) >= 2 {
				vxlan.Port = binary.BigEndian.Uint16(buf)
			}
		case unix.IFLA_VXLAN_LEARNING:
			vxlan.Learning = decoder.Uint8() != 0
		}
	}

	return decoder.Err()
}
