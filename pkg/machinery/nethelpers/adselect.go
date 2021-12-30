// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate enumer -type=ADSelect -linecomment -text

// ADSelect is ADSelect.
type ADSelect uint8

// ADSelect constants.
const (
	ADSelectStable    ADSelect = iota // stable
	ADSelectBandwidth                 // bandwidth
	ADSelectCount                     // count
)

// ADSelectByName parses ADSelect.
func ADSelectByName(sel string) (ADSelect, error) {
	switch sel {
	case "", "stable":
		return ADSelectStable, nil
	case "bandwidth":
		return ADSelectBandwidth, nil
	case "count":
		return ADSelectCount, nil
	default:
		return 0, fmt.Errorf("invalid ad_select mode %v", sel)
	}
}
