// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/mdlayher/netlink"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const vethInfoPeer = 1

// VethSpec provides encoding helpers for network.VethSpec.
func VethSpec(r *network.VethSpec) vethSpec {
	return vethSpec{VethSpec: r}
}

type vethSpec struct {
	*network.VethSpec
}

// Encode encodes VETH_INFO_PEER data for a link creation request.
func (spec vethSpec) Encode() ([]byte, error) {
	if spec.PeerName == "" {
		return nil, fmt.Errorf("veth peer name must be specified")
	}

	peer, err := (&rtnetlink.LinkMessage{
		Attributes: &rtnetlink.LinkAttributes{
			Name: spec.PeerName,
		},
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("error encoding veth peer link: %w", err)
	}

	encoder := netlink.NewAttributeEncoder()
	encoder.Bytes(vethInfoPeer, peer)

	data, err := encoder.Encode()
	if err != nil {
		return nil, fmt.Errorf("error encoding veth peer attribute: %w", err)
	}

	return data, nil
}
