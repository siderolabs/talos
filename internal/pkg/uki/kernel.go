// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"strings"
)

// DiscoverKernelVersion reads kernel version from the kernel image.
//
// This only works for x86 kernel images.
//
// Based on https://www.kernel.org/doc/html/v5.6/x86/boot.html.
func DiscoverKernelVersion(kernelPath string) (string, error) {
	f, err := os.Open(kernelPath)
	if err != nil {
		return "", err
	}

	defer f.Close() //nolint:errcheck

	header := make([]byte, 1024)

	_, err = f.Read(header)
	if err != nil {
		return "", err
	}

	// check header magic
	if string(header[0x202:0x206]) != "HdrS" {
		return "", errors.New("invalid kernel image")
	}

	setupSects := header[0x1f1]
	versionOffset := binary.LittleEndian.Uint16(header[0x20e:0x210])

	if versionOffset == 0 {
		return "", errors.New("no kernel version")
	}

	if versionOffset > uint16(setupSects)*0x200 {
		return "", errors.New("invalid kernel version offset")
	}

	versionOffset += 0x200

	version := make([]byte, 256)

	_, err = f.ReadAt(version, int64(versionOffset))
	if err != nil {
		return "", err
	}

	idx := bytes.IndexByte(version, 0)
	if idx == -1 {
		return "", errors.New("invalid kernel version")
	}

	versionString := string(version[:idx])
	versionString, _, _ = strings.Cut(versionString, " ")

	return versionString, nil
}
