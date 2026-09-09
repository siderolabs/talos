// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysblock

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"

	blkdev "github.com/siderolabs/go-blockdevice/v2/block"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// ParentDeviceKey is a uevent value Talos fills in for a device whose parent cannot be derived from
// its place in sysfs: the kernel device name of the parent device.
//
// A partition is normally a sysfs directory inside the directory of the disk it belongs to, which is
// where its parent is read from. A device-mapper partition map instead sits next to its parent under
// /sys/devices/virtual/block, so the parent has to be named explicitly.
const ParentDeviceKey = "TALOS_PARENT_DEVNAME"

// AugmentDeviceMapper fills in the uevent values the kernel does not provide for a device-mapper
// partition map, so that the rest of the system sees it as the partition it is.
//
// The kernel announces every device-mapper device as a whole disk and never as a partition: it sets
// GENHD_FL_NO_PART on device-mapper gendisks, so a device-mapper disk has no kernel partitions at
// all. The partitions of such a disk are separate device-mapper devices mapped over it, identified
// by the UUID "part<N>-<parent UUID>" as kpartx composes it.
//
// A device is only taken for a partition map if the device it is mapped over is the device-mapper
// device its UUID names. kpartx run over something which is not device-mapper - a loop device, say,
// where it writes "part1-devnode_7:0_..." - is left alone, because that device carries real kernel
// partitions of its own and a second view of them would be a duplicate.
func AugmentDeviceMapper(devicePath string, values map[string]string) {
	if values["DEVTYPE"] != block.DeviceTypeDisk {
		return
	}

	partitionNumber, parentUUID, ok := blkdev.ParseDeviceMapperPartitionUUID(readSysFsFile(devicePath, "dm", "uuid"))
	if !ok {
		return
	}

	// a partition map is mapped over exactly one device: the disk it is a partition of
	slaves := ReadSecondaries(devicePath)
	if len(slaves) != 1 {
		return
	}

	parent := slaves[0]

	if readSysFsFile(devicePath, "slaves", parent, "dm", "uuid") != parentUUID {
		return
	}

	values["DEVTYPE"] = block.DeviceTypePartition
	values["PARTN"] = strconv.FormatUint(uint64(partitionNumber), 10)
	values[ParentDeviceKey] = parent
}

// readSysFsFile reads a sysfs attribute with the trailing whitespace trimmed.
//
// An attribute which cannot be read, most often one which is not there for this device, reads as
// empty.
func readSysFsFile(path ...string) string {
	contents, err := os.ReadFile(filepath.Join(path...))
	if err != nil {
		return ""
	}

	return string(bytes.TrimSpace(contents))
}
