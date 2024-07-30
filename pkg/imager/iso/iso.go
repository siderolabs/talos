// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package iso contains functions for creating ISO images.
package iso

import "strings"

// VolumeID returns a valid volume ID for the given label.
func VolumeID(label string) string {
	// builds a valid volume ID: 32 chars out of [A-Z0-9_]
	label = strings.ToUpper(label)
	label = strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_' || r == '-' || r == '.':
			return '_'
		default:
			return -1
		}
	}, label)

	if len(label) > 32 {
		label = label[:32]
	}

	return label
}

// Label returns an ISO full label for a given version.
func Label(version string, secureboot bool) string {
	label := "Talos-"

	if secureboot {
		label += "SB-"
	}

	return label + version
}
