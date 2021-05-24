// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "fmt"

//go:generate stringer -type=ADSelect -linecomment

// ADSelect is ADSelect.
type ADSelect uint8

// MarshalYAML implements yaml.Marshaler.
func (v ADSelect) MarshalYAML() (interface{}, error) {
	return v.String(), nil
}

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
