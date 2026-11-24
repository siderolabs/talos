// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sysfs reads attributes from the sysfs filesystem.
package sysfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
)

// ErrInvalidReadCount is returned when a read reports a byte count outside the buffer it was given.
//
// A sysfs show() callback returning a negative value which is not an errno makes read(2) return
// it as is, and the Go standard library passes it through as the byte count, so os.ReadFile and
// io.ReadAll panic with "slice bounds out of range".
var ErrInvalidReadCount = errors.New("invalid read count")

// readChunk is the read size, a sysfs attribute fits into a page.
const readChunk = 4096

// ReadFile reads a sysfs attribute.
//
// Unlike os.ReadFile, it returns an error instead of panicking when the kernel returns a bogus
// byte count.
func ReadFile(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	data, err := ReadAll(f)
	if errors.Is(err, ErrInvalidReadCount) {
		err = &fs.PathError{Op: "read", Path: name, Err: err}
	}

	return data, err
}

// ReadAll reads from r until EOF.
//
// Unlike io.ReadAll, it returns ErrInvalidReadCount instead of panicking when r returns a byte
// count outside the buffer it was given.
func ReadAll(r io.Reader) ([]byte, error) {
	data := make([]byte, 0, readChunk)

	for {
		if len(data) == cap(data) {
			data = slices.Grow(data, readChunk)
		}

		n, err := r.Read(data[len(data):cap(data)])
		if n < 0 || n > cap(data)-len(data) {
			return nil, fmt.Errorf("%w: %d", ErrInvalidReadCount, n)
		}

		data = data[:len(data)+n]

		if err != nil {
			if errors.Is(err, io.EOF) {
				return data, nil
			}

			return nil, err
		}
	}
}
