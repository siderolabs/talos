// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rng

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// GetPoolSize returns kernel random pool size.
func GetPoolSize() (int, error) {
	contents, err := os.ReadFile("/proc/sys/kernel/random/poolsize")
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(bytes.TrimSpace(contents)))
}

// WriteEntropy writes entropy data to the pool.
func WriteEntropy(data []byte) error {
	fd, err := os.OpenFile("/dev/urandom", os.O_WRONLY|unix.O_CLOEXEC|unix.O_NOCTTY, 0)
	if err != nil {
		return err
	}

	defer fd.Close() //nolint:errcheck

	_, err = fd.Write(data)
	if err != nil {
		return fmt.Errorf("error writing entropy: %w", err)
	}

	return fd.Close()
}
