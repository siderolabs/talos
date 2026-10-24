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

// BlockDeviceAddresses returns the addresses of the PCI devices a block device is on, read from
// sysfs alone: a partition resolves to its disk, a device-mapper device to the devices it is built
// on, and the namespace head of native NVMe multipath to the controllers of its paths.
//
// It reports false while that cannot be known yet: the device is not in sysfs, or a multipath
// head or device-mapper device has no path or slave yet. A device on no PCI device (e.g. a loop
// device) yields no address and true.
func BlockDeviceAddresses(sysfsPath, name string) ([]string, bool) {
	link := filepath.Join(sysfsPath, "class", "block", name)

	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		return nil, false
	}

	if _, err = os.Stat(filepath.Join(link, "partition")); err == nil {
		return BlockDeviceAddresses(sysfsPath, filepath.Base(filepath.Dir(resolved)))
	}

	if address := AddressFromBusPath(resolved); address != "" {
		return []string{address}, true
	}

	// A native NVMe multipath head is reached only through its paths: one with none left (its
	// controllers did not enumerate) cannot be placed.
	_, mpErr := os.Stat(filepath.Join(link, "multipath"))
	if mpErr == nil || strings.Contains(resolved, "/nvme-subsystem/") {
		addresses := DiskAddresses(sysfsPath, name, "")

		return addresses, len(addresses) > 0
	}

	return slaveAddresses(sysfsPath, link)
}

// slaveAddresses returns the addresses of the PCI devices under the block devices a block device
// is built on (e.g. a device-mapper device's slaves).
func slaveAddresses(sysfsPath, link string) ([]string, bool) {
	slaves, err := os.ReadDir(filepath.Join(link, "slaves"))
	if err != nil || len(slaves) == 0 {
		// Not built on other block devices; device-mapper devices always are, once set up.
		_, dmErr := os.Stat(filepath.Join(link, "dm"))

		return nil, dmErr != nil
	}

	var addresses []string

	for _, slave := range slaves {
		slaveAddresses, ok := BlockDeviceAddresses(sysfsPath, slave.Name())
		if !ok {
			return nil, false
		}

		for _, address := range slaveAddresses {
			if !slices.Contains(addresses, address) {
				addresses = append(addresses, address)
			}
		}
	}

	return addresses, true
}
