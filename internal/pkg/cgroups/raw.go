// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroups

import (
	"bufio"
	"io"
)

// RawValue is a raw cgroup value (without any parsing).
type RawValue string

// ParseRawValue parses the raw cgroup value.
func ParseRawValue(r io.Reader) (RawValue, error) {
	scanner := bufio.NewScanner(r)

	if !scanner.Scan() {
		return RawValue(""), nil
	}

	line := scanner.Text()

	if err := scanner.Err(); err != nil {
		return RawValue(""), err
	}

	return RawValue(line), nil
}
