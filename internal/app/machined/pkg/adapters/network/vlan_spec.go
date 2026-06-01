// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"encoding/binary"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// VLANSpec adapter provides encoding/decoding to netlink structures.
//
//nolint:revive,golint
func VLANSpec(r *network.VLANSpec) vlanSpec {
	return vlanSpec{
		VLANSpec: r,
	}
}

type vlanSpec struct {
	*network.VLANSpec
}

// Encode the VLANSpec into netlink attributes.
func (a vlanSpec) Encode() ([]byte, error) {
	vlan := a.VLANSpec

	encoder := netlink.NewAttributeEncoder()

	encoder.Uint16(unix.IFLA_VLAN_ID, vlan.VID)

	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(vlan.Protocol))
	encoder.Bytes(unix.IFLA_VLAN_PROTOCOL, buf)

	return encoder.Encode()
}

// Decode the VLANSpec from netlink attributes.
func (a vlanSpec) Decode(data []byte) error {
	vlan := a.VLANSpec

	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		switch decoder.Type() {
		case unix.IFLA_VLAN_ID:
			vlan.VID = decoder.Uint16()
		case unix.IFLA_VLAN_PROTOCOL:
			vlan.Protocol = nethelpers.VLANProtocol(binary.BigEndian.Uint16(decoder.Bytes()))
		}
	}

	return decoder.Err()
}
