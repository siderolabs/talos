// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package wireguard implements common wireguard functionality.
package wireguard

import "time"

// PeerDownInterval is the time since last handshake when established peer is considered to be down.
//
// WG whitepaper defines a downed peer as being:
// Handshake Timeout (180s) + Rekey Timeout (5s) + Rekey Attempt Timeout (90s)
//
// This interval is applied when the link is already established.
const PeerDownInterval = (180 + 5 + 90) * time.Second
