// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseMillicores parses a Kubernetes-style CPU quantity in millicores.
//
// Only the `<n>m` form is accepted. Bare core counts are rejected rather than guessed at, since
// `1` meaning one core and `1` meaning one millicore are an easy and expensive confusion.
func ParseMillicores(value string) (uint64, error) {
	if !strings.HasSuffix(value, "m") {
		return 0, fmt.Errorf("%q must be expressed in millicores, e.g. 1500m", value)
	}

	millicores, err := strconv.ParseUint(strings.TrimSuffix(value, "m"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a valid millicore quantity: %w", value, err)
	}

	if millicores == 0 {
		return 0, fmt.Errorf("%q must be greater than zero", value)
	}

	return millicores, nil
}
