// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=FailOverMAC -linecomment

// FailOverMAC is a MAC failover mode.
type FailOverMAC uint8

// FailOverMAC constants.
const (
	FailOverMACNone   FailOverMAC = iota // none
	FailOverMACActive                    // active
	FailOverMACFollow                    // follow
)

// FailOverMACByName parses FailOverMac.
func FailOverMACByName(f string) (FailOverMAC, error) {
	switch f {
	case "", "none":
		return FailOverMACNone, nil
	case "active":
		return FailOverMACActive, nil
	case "follow":
		return FailOverMACFollow, nil
	default:
		return 0, fmt.Errorf("invalid fail_over_mac value %v", f)
	}
}
