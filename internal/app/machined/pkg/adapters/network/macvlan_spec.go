// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Encode/Decode below mirror vrf_master_spec.go: both carry a single uint32 attribute.
//
//nolint:dupl
package network

import (
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// MacVLANSpec adapter provides encoding/decoding to netlink structures.
//
// The attributes are encoded/decoded by hand (like the other adapters in this package) on purpose:
// github.com/jsimonetti/rtnetlink/v2/driver registers its drivers in a global rtnetlink registry from
// init(), which makes rtnetlink decode IFLA_INFO_DATA into typed driver structs instead of raw bytes
// for every kind it knows about, breaking rawLinkData for bonds, bridges, VLANs, macvlans and veths.
//
//nolint:revive
func MacVLANSpec(r *network.MacVLANSpec) macvlanSpec {
	return macvlanSpec{
		MacVLANSpec: r,
	}
}

type macvlanSpec struct {
	*network.MacVLANSpec
}

// Encode the MacVLANSpec into netlink attributes.
func (a macvlanSpec) Encode() ([]byte, error) {
	macvlan := a.MacVLANSpec

	encoder := netlink.NewAttributeEncoder()

	encoder.Uint32(unix.IFLA_MACVLAN_MODE, uint32(macvlan.Mode))

	return encoder.Encode()
}

// Decode the MacVLANSpec from netlink attributes.
func (a macvlanSpec) Decode(data []byte) error {
	macvlan := a.MacVLANSpec

	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		if decoder.Type() == unix.IFLA_MACVLAN_MODE {
			macvlan.Mode = nethelpers.MacvlanMode(decoder.Uint32())
		}
	}

	return decoder.Err()
}
