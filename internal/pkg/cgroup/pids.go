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
)

// PidsMaxFile is the cgroup v2 interface file holding the hard limit on the number of tasks.
const PidsMaxFile = "pids.max"

const pidsMaxUnlimited = "max"

// PidsMax is a parsed pids.max value.
type PidsMax struct {
	Value     uint64
	Unlimited bool
}

// UnlimitedPidsMax returns the unlimited pids.max value.
func UnlimitedPidsMax() PidsMax {
	return PidsMax{Unlimited: true}
}

// LimitedPidsMax returns a finite pids.max value.
func LimitedPidsMax(value uint64) PidsMax {
	return PidsMax{Value: value}
}

// String formats the value exactly as it is written to pids.max.
func (p PidsMax) String() string {
	if p.Unlimited {
		return pidsMaxUnlimited
	}

	return strconv.FormatUint(p.Value, 10)
}

// ReadPidsMax reads and parses pids.max of the cgroup directory.
func ReadPidsMax(dir string) (PidsMax, error) {
	path := filepath.Join(dir, PidsMaxFile)

	contents, err := os.ReadFile(path)
	if err != nil {
		return PidsMax{}, fmt.Errorf("error reading %s: %w", path, err)
	}

	value, err := parsePidsMax(string(contents))
	if err != nil {
		return PidsMax{}, fmt.Errorf("error parsing %s: %w", path, err)
	}

	return value, nil
}

func parsePidsMax(contents string) (PidsMax, error) {
	contents = strings.TrimSpace(contents)

	if contents == pidsMaxUnlimited {
		return UnlimitedPidsMax(), nil
	}

	value, err := strconv.ParseUint(contents, 10, 64)
	if err != nil {
		return PidsMax{}, fmt.Errorf("unexpected value %q: %w", contents, err)
	}

	return LimitedPidsMax(value), nil
}

// WritePidsMax writes the value to pids.max of the cgroup directory.
//
// Kernfs ignores O_TRUNC; regular-file test fixtures need it when the value becomes shorter.
func WritePidsMax(dir string, value PidsMax) error {
	path := filepath.Join(dir, PidsMaxFile)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", path, err)
	}

	_, writeErr := f.WriteString(value.String())

	if err = errors.Join(writeErr, f.Close()); err != nil {
		return fmt.Errorf("error writing %s to %s: %w", value, path, err)
	}

	return nil
}

// EnsurePidsMax makes pids.max of the cgroup directory hold the desired value.
//
// It returns the value found before any write and whether a write happened.
func EnsurePidsMax(dir string, desired PidsMax) (PidsMax, bool, error) {
	return ensureLimit(dir, PidsMaxFile, desired, ReadPidsMax, WritePidsMax)
}

// PidCapacity returns the number of tasks the host can run: min(kernel.pid_max, kernel.threads-max).
//
// procSys is the sysctl mount point, normally /proc/sys. This matches the PID capacity the kubelet
// derives for Node Allocatable.
func PidCapacity(procSys string) (uint64, error) {
	var capacity uint64

	for i, name := range []string{"pid_max", "threads-max"} {
		path := filepath.Join(procSys, "kernel", name)

		contents, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("error reading %s: %w", path, err)
		}

		value, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing %s: %w", path, err)
		}

		if i == 0 || value < capacity {
			capacity = value
		}
	}

	return capacity, nil
}

// AllocatablePids returns the PID capacity minus the reservation, floored at zero, as kubelet
// Node Allocatable does.
func AllocatablePids(capacity, reserved uint64) uint64 {
	if capacity <= reserved {
		return 0
	}

	return capacity - reserved
}
