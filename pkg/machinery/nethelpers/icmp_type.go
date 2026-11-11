// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// ICMPType is a ICMP or ICMPv6 packet type.
//
// ICMP and ICMPv6 type numbers overlap, so the constant should be used with the matching protocol.
type ICMPType byte

// ICMPType constants.
//
//structprotogen:gen_enum
const (
	ICMPTypeTimestampRequest   ICMPType = 13 // timestamp-request
	ICMPTypeTimestampReply     ICMPType = 14 // timestamp-reply
	ICMPTypeAddressMaskRequest ICMPType = 17 // address-mask-request
	ICMPTypeAddressMaskReply   ICMPType = 18 // address-mask-reply

	// ICMPv6 Neighbor Discovery message types (RFC 4861).
	ICMPv6TypeNeighborSolicitation  ICMPType = 135 // nd-neighbor-solicit
	ICMPv6TypeNeighborAdvertisement ICMPType = 136 // nd-neighbor-advert
	ICMPv6TypeRedirect              ICMPType = 137 // nd-redirect
)
