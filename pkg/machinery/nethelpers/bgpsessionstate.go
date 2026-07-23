// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// BGPSessionState is the state of a BGP peering session (RFC 4271 FSM).
type BGPSessionState uint8

// BGPSessionState constants.
//
//structprotogen:gen_enum
const (
	BGPSessionStateUnknown     BGPSessionState = iota // UNKNOWN
	BGPSessionStateIdle                               // IDLE
	BGPSessionStateConnect                            // CONNECT
	BGPSessionStateActive                             // ACTIVE
	BGPSessionStateOpenSent                           // OPEN_SENT
	BGPSessionStateOpenConfirm                        // OPEN_CONFIRM
	BGPSessionStateEstablished                        // ESTABLISHED
)
