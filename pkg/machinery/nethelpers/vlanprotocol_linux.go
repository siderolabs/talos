// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "golang.org/x/sys/unix"

//go:generate stringer -type=VLANProtocol -linecomment -output vlanprotocol_string_linux.go

// VLANProtocol is a VLAN protocol.
type VLANProtocol uint16

// MarshalYAML implements yaml.Marshaler.
func (proto VLANProtocol) MarshalYAML() (interface{}, error) {
	return proto.String(), nil
}

// VLANProtocol constants.
const (
	VLANProtocol8021Q  VLANProtocol = unix.ETH_P_8021Q  // 802.1q
	VLANProtocol8021AD VLANProtocol = unix.ETH_P_8021AD // 802.1ad
)
