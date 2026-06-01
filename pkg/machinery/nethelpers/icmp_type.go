// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// ICMPType is a ICMP packet type.
type ICMPType byte

// ICMPType constants.
//
//structprotogen:gen_enum
const (
	ICMPTypeTimestampRequest   ICMPType = 13 // timestamp-request
	ICMPTypeTimestampReply     ICMPType = 14 // timestamp-reply
	ICMPTypeAddressMaskRequest ICMPType = 17 // address-mask-request
	ICMPTypeAddressMaskReply   ICMPType = 18 // address-mask-reply
)
