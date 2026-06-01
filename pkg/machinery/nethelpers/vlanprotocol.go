// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// VLANProtocol is a VLAN protocol.
type VLANProtocol uint16

// VLANProtocol constants.
//
//structprotogen:gen_enum
const (
	VLANProtocol8021Q  VLANProtocol = 33024 // 802.1q
	VLANProtocol8021AD VLANProtocol = 34984 // 802.1ad
)
