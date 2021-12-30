// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=LACPRate -linecomment -text

// LACPRate is a LACP rate.
type LACPRate uint8

// LACP rate constants.
const (
	LACPRateSlow LACPRate = iota // slow
	LACPRateFast                 // fast
)

// LACPRateByName parses LACPRate.
func LACPRateByName(mode string) (LACPRate, error) {
	switch mode {
	case "", "slow":
		return LACPRateSlow, nil
	case "fast":
		return LACPRateFast, nil
	default:
		return 0, fmt.Errorf("invalid lacp rate %v", mode)
	}
}
