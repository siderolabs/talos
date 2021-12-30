// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=ARPAllTargets -linecomment -text

// ARPAllTargets is an ARP targets mode.
type ARPAllTargets uint32

// ARPAllTargets contants.
const (
	ARPAllTargetsAny ARPAllTargets = iota // any
	ARPAllTargetsAll                      // all
)

// ARPAllTargetsByName parses ARPAllTargets.
func ARPAllTargetsByName(a string) (ARPAllTargets, error) {
	switch a {
	case "", "any":
		return ARPAllTargetsAny, nil
	case "all":
		return ARPAllTargetsAll, nil
	default:
		return 0, fmt.Errorf("invalid arp_all_targets mode %v", a)
	}
}
