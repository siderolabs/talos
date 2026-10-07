// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// MemoryMaxFile is the cgroup v2 interface file holding the hard memory limit.
const MemoryMaxFile = "memory.max"

const memoryMaxUnlimited = "max"

// MemoryMax is a parsed memory.max value.
type MemoryMax struct {
	Bytes     uint64
	Unlimited bool
}

// UnlimitedMemoryMax returns the unlimited memory.max value.
func UnlimitedMemoryMax() MemoryMax {
	return MemoryMax{Unlimited: true}
}

// LimitedMemoryMax returns a finite memory.max value; bytes should come from kernel.NormalizeMemoryLimit.
func LimitedMemoryMax(bytes uint64) MemoryMax {
	return MemoryMax{Bytes: bytes}
}

// String formats the value exactly as it is written to memory.max.
func (m MemoryMax) String() string {
	if m.Unlimited {
		return memoryMaxUnlimited
	}

	return strconv.FormatUint(m.Bytes, 10)
}

// ReadMemoryMax reads and parses memory.max of the cgroup directory.
func ReadMemoryMax(dir string) (MemoryMax, error) {
	path := filepath.Join(dir, MemoryMaxFile)

	contents, err := os.ReadFile(path)
	if err != nil {
		return MemoryMax{}, fmt.Errorf("error reading %s: %w", path, err)
	}

	value, err := parseMemoryMax(string(contents))
	if err != nil {
		return MemoryMax{}, fmt.Errorf("error parsing %s: %w", path, err)
	}

	return value, nil
}

func parseMemoryMax(contents string) (MemoryMax, error) {
	contents = strings.TrimSpace(contents)

	if contents == memoryMaxUnlimited {
		return UnlimitedMemoryMax(), nil
	}

	bytes, err := strconv.ParseUint(contents, 10, 64)
	if err != nil {
		return MemoryMax{}, fmt.Errorf("unexpected value %q: %w", contents, err)
	}

	return LimitedMemoryMax(bytes), nil
}

// WriteMemoryMax writes the value to memory.max of the cgroup directory.
//
// O_NONBLOCK avoids reclaim/OOM work in the writer on Linux 6.16+; usage can remain above
// the new limit until subsequent charges. Older kernels ignore the flag.
//
// Kernfs ignores O_TRUNC; regular-file test fixtures need it when the value becomes shorter.
func WriteMemoryMax(dir string, value MemoryMax) error {
	path := filepath.Join(dir, MemoryMaxFile)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|unix.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", path, err)
	}

	_, writeErr := f.WriteString(value.String())

	if err = errors.Join(writeErr, f.Close()); err != nil {
		return fmt.Errorf("error writing %s to %s: %w", value, path, err)
	}

	return nil
}

// EnsureMemoryMax makes memory.max of the cgroup directory hold the desired value.
//
// Finite desired values must have gone through kernel.NormalizeMemoryLimit.
// It returns the value found before any write and whether a write happened.
func EnsureMemoryMax(dir string, desired MemoryMax) (MemoryMax, bool, error) {
	current, err := ReadMemoryMax(dir)
	if err != nil {
		return MemoryMax{}, false, err
	}

	if current == desired {
		return current, false, nil
	}

	if err = WriteMemoryMax(dir, desired); err != nil {
		return current, false, err
	}

	actual, err := ReadMemoryMax(dir)
	if err != nil {
		return current, true, fmt.Errorf("error verifying write: %w", err)
	}

	if actual != desired {
		return current, true, fmt.Errorf("%s reads back %s after writing %s", filepath.Join(dir, MemoryMaxFile), actual, desired)
	}

	return current, true, nil
}
