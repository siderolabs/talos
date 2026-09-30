// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PTPDevicePathPrefix is the prefix of the time server entry which represents a PTP device.
const PTPDevicePathPrefix = "/dev/"

// IsPTPDevicePath checks if a given time server entry is meant to be a PTP device.
//
// It doesn't validate the path, use ValidatePTPDevicePath for that.
func IsPTPDevicePath(server string) bool {
	return strings.HasPrefix(server, PTPDevicePathPrefix)
}

// ValidatePTPDevicePath validates that the path points to a PTP device node.
//
// A valid PTP device path is a clean path directly under /dev with the name starting with "ptp",
// e.g. /dev/ptp0, /dev/ptp_kvm, /dev/ptp_hyperv.
func ValidatePTPDevicePath(path string) error {
	if filepath.Clean(path) != path {
		return fmt.Errorf("PTP device path %q is not clean", path)
	}

	if filepath.Dir(path) != filepath.Clean(PTPDevicePathPrefix) {
		return fmt.Errorf("PTP device path %q should be directly under %s", path, PTPDevicePathPrefix)
	}

	if !strings.HasPrefix(filepath.Base(path), "ptp") {
		return fmt.Errorf("PTP device path %q should have a name starting with 'ptp'", path)
	}

	return nil
}
