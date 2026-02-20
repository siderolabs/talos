// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// VRFMasterSpec adapter provides encoding/decoding to netlink structures.
//
//nolint:revive
func VRFMasterSpec(r *network.VRFMasterSpec) vrfMaster {
	return vrfMaster{
		VRFMasterSpec: r,
	}
}

// vrfMaster contains the vrf master spec and provides methods for encoding/decoding it to netlink structures.
type vrfMaster struct {
	*network.VRFMasterSpec
}

// Encode the VRFMasterSpec into netlink attributes.
func (a vrfMaster) Encode() ([]byte, error) {
	vrf := a.VRFMasterSpec

	encoder := netlink.NewAttributeEncoder()

	encoder.Uint32(unix.IFLA_VRF_TABLE, uint32(vrf.Table))

	return encoder.Encode()
}

// Decode the VRFMasterSpec from netlink attributes.
func (a vrfMaster) Decode(data []byte) error {
	vrf := a.VRFMasterSpec

	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		if decoder.Type() == unix.IFLA_VRF_TABLE {
			vrf.Table = nethelpers.RoutingTable(decoder.Uint32())
		}
	}

	return decoder.Err()
}
