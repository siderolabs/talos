// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=VLANProtocol -linecomment

// VLANProtocol is a VLAN protocol.
type VLANProtocol uint16

// MarshalYAML implements yaml.Marshaler.
func (proto VLANProtocol) MarshalYAML() (interface{}, error) {
	return proto.String(), nil
}

// VLANProtocol constants.
const (
	VLANProtocol8021Q  VLANProtocol = 33024 // 802.1q
	VLANProtocol8021AD VLANProtocol = 34984 // 802.1ad
)
