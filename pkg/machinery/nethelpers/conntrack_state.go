// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// ConntrackState is a conntrack state.
type ConntrackState uint32

// ConntrackState constants.
//
//structprotogen:gen_enum
const (
	ConntrackStateNew         ConntrackState = 0x08 // new
	ConntrackStateRelated     ConntrackState = 0x04 // related
	ConntrackStateEstablished ConntrackState = 0x02 // established
	ConntrackStateInvalid     ConntrackState = 0x01 // invalid
)
