// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pci

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
)

const sysfsPath = "/sys/bus/pci/devices/%s/%s"

func readID(busPath, name string) (uint16, error) {
	contents, err := os.ReadFile(fmt.Sprintf(sysfsPath, busPath, name))
	if err != nil {
		return 0, err
	}

	v, err := strconv.ParseUint(string(bytes.TrimSpace(contents)), 0, 16)

	return uint16(v), err
}

// SysfsDeviceInfo looks up vendor and product ID from sysfs.
func SysfsDeviceInfo(busPath string) (*Device, error) {
	var (
		d   Device
		err error
	)

	d.ProductID, err = readID(busPath, "device")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	d.VendorID, err = readID(busPath, "vendor")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	return &d, err
}
