// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=PrimaryReselect -linecomment -text

// PrimaryReselect is an ARP targets mode.
type PrimaryReselect uint8

// PrimaryReslect constants.
const (
	PrimaryReselectAlways  PrimaryReselect = iota // always
	PrimaryReselectBetter                         // better
	PrimaryReselectFailure                        // failure
)

// PrimaryReselectByName parses PrimaryReselect.
func PrimaryReselectByName(p string) (PrimaryReselect, error) {
	switch p {
	case "", "always":
		return PrimaryReselectAlways, nil
	case "better":
		return PrimaryReselectBetter, nil
	case "failure":
		return PrimaryReselectFailure, nil
	default:
		return 0, fmt.Errorf("invalid primary_reselect mode %v", p)
	}
}
