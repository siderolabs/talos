// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// DeviceInfo contains device hardware information that can be read from /sys/.
type DeviceInfo struct {
	BusPath string
	PCIID   string
	Driver  string
}

// GetDeviceInfo get additional device information by reading /sys/ directory.
//nolint:gocyclo
func GetDeviceInfo(deviceName string) (*DeviceInfo, error) {
	path := filepath.Join("/sys/class/net/", deviceName, "/device/")

	readFile := func(path string) (string, error) {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}

		res, err := ioutil.ReadAll(f)
		if err != nil {
			return "", err
		}

		return string(bytes.TrimSpace(res)), nil
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &DeviceInfo{}, nil
		}

		return nil, err
	}

	ueventContents, err := readFile(filepath.Join(path, "uevent"))
	if err != nil {
		return nil, err
	}

	if ueventContents == "" {
		return &DeviceInfo{}, nil
	}

	device := &DeviceInfo{}

	for _, line := range strings.Split(ueventContents, "\n") {
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "DRIVER":
			device.Driver = value
		case "PCI_ID":
			device.PCIID = value
		case "PCI_SLOT_NAME":
			device.BusPath = value
		}
	}

	return device, nil
}
