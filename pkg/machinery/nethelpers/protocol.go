// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate enumer -type=Protocol -linecomment -text

// Protocol is a inet protocol.
type Protocol uint8

// Protocol constants.
//
//structprotogen:gen_enum
const (
	ProtocolTCP Protocol = 0x6  // tcp
	ProtocolUDP Protocol = 0x11 // udp
)
