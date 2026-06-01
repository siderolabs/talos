// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"strings"
)

var okays = []string{"y", "yes"}

// Confirm asks the user to confirm their action. Anything other than
// `y` and `yes` returns false.
func Confirm(prompt string) bool {
	var inp string

	fmt.Printf("%s (y/N): ", prompt)
	fmt.Scanf("%s", &inp) //nolint:errcheck
	inp = strings.TrimSpace(inp)

	for _, ok := range okays {
		if strings.EqualFold(inp, ok) {
			return true
		}
	}

	return false
}
