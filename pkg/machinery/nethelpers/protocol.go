// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// Protocol is a inet protocol.
type Protocol uint8

// Protocol constants.
//
//structprotogen:gen_enum
const (
	ProtocolICMP   Protocol = 0x1  // icmp
	ProtocolTCP    Protocol = 0x6  // tcp
	ProtocolUDP    Protocol = 0x11 // udp
	ProtocolICMPv6 Protocol = 0x3a // icmpv6
)
