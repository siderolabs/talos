// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup

import (
	"fmt"
	"path/filepath"
)

// limitValue is a parsed value of a single-value cgroup interface file, such as memory.max or pids.max.
type limitValue interface {
	comparable
	fmt.Stringer
}

// ensureLimit makes the cgroup interface file hold the desired value.
//
// The file is read first and written only if it differs; the write is then verified by reading it back.
// It returns the value found before any write and whether a write happened.
func ensureLimit[T limitValue](
	dir, file string,
	desired T,
	read func(dir string) (T, error),
	write func(dir string, value T) error,
) (T, bool, error) {
	var zero T

	current, err := read(dir)
	if err != nil {
		return zero, false, err
	}

	if current == desired {
		return current, false, nil
	}

	if err = write(dir, desired); err != nil {
		return current, false, err
	}

	actual, err := read(dir)
	if err != nil {
		return current, true, fmt.Errorf("error verifying write: %w", err)
	}

	if actual != desired {
		return current, true, fmt.Errorf("%s reads back %s after writing %s", filepath.Join(dir, file), actual, desired)
	}

	return current, true, nil
}
