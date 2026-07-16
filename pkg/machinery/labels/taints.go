// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package labels

import "strings"

// ParseTaint parses a taint value from a string representation.
func ParseTaint(s string) (value, effect string) {
	var found bool

	value, effect, found = strings.Cut(s, ":")
	if !found {
		effect = value
		value = ""
	}

	return value, effect
}
