// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pci

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var addressRegexp = regexp.MustCompile(`^[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\.[0-7]$`)

// AddressFromBusPath returns the address of the PCI device a device sits on, from its bus path
// (e.g. "/pci0000:80/0000:80:05.1/0000:85:00.0/nvme" yields "0000:85:00.0"), or an empty string
// when it is not on a PCI bus.
//
// The last PCI address in the path is the device itself; the ones before it are the bridges
// leading to it.
func AddressFromBusPath(busPath string) string {
	var address string

	for component := range strings.SplitSeq(busPath, "/") {
		if addressRegexp.MatchString(component) {
			address = component
		}
	}

	return address
}

// DiskAddresses returns the addresses of the PCI devices a block device is on.
//
// The address comes from the disk's bus path when it is on a PCI device. A disk without one may be
// the namespace head of native NVMe multipath, which sits under the virtual nvme-subsystem device;
// its paths, listed under "multipath" in sysfs, lead to the controllers it is reached through.
func DiskAddresses(sysfsPath, disk, busPath string) []string {
	if address := AddressFromBusPath(busPath); address != "" {
		return []string{address}
	}

	paths, err := os.ReadDir(filepath.Join(sysfsPath, "class", "block", disk, "multipath"))
	if err != nil {
		return nil
	}

	var addresses []string

	for _, path := range paths {
		resolved, err := filepath.EvalSymlinks(filepath.Join(sysfsPath, "class", "block", disk, "multipath", path.Name()))
		if err != nil {
			continue
		}

		if address := AddressFromBusPath(resolved); address != "" && !slices.Contains(addresses, address) {
			addresses = append(addresses, address)
		}
	}

	return addresses
}
