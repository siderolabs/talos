// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package xfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// AsOSFile attempts to convert fs.File to *os.File.
func AsOSFile(f fs.File, name string) (*os.File, error) {
	ff, ok := f.(File)
	if !ok {
		return nil, errors.ErrUnsupported
	}

	newFd, err := syscall.Dup(int(ff.Fd()))
	if err != nil {
		return nil, fmt.Errorf("failed to duplicate file descriptor: %w", err)
	}

	return os.NewFile(uintptr(newFd), name), nil
}
