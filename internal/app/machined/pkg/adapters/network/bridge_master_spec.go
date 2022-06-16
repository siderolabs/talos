// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// BridgeMasterSpec adapter provides encoding/decoding to netlink structures.
//
//nolint:revive,golint
func BridgeMasterSpec(r *network.BridgeMasterSpec) bridgeMaster {
	return bridgeMaster{
		BridgeMasterSpec: r,
	}
}

type bridgeMaster struct {
	*network.BridgeMasterSpec
}

// Encode the BridgeMasterSpec into netlink attributes.
//
//nolint:gocyclo
func (a bridgeMaster) Encode() ([]byte, error) {
	bridge := a.BridgeMasterSpec

	encoder := netlink.NewAttributeEncoder()

	stpEnabled := 0
	if bridge.STPEnabled {
		stpEnabled = 1
	}

	encoder.Uint32(unix.IFLA_BR_STP_STATE, uint32(stpEnabled))

	return encoder.Encode()
}

// Decode the BridgeMasterSpec from netlink attributes.
//
//nolint:gocyclo,cyclop
func (a bridgeMaster) Decode(data []byte) error {
	bridge := a.BridgeMasterSpec

	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		if decoder.Type() == unix.IFLA_BR_STP_STATE {
			bridge.STPEnabled = decoder.Uint32() != 0
		}
	}

	return decoder.Err()
}
