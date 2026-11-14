// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysblock

import (
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// multipathUUIDPrefix is the prefix of the device-mapper UUID of a multipath device, as
// multipath-tools composes it: "mpath-<WWID>".
//
// The partition maps of a multipath device carry "part<N>-mpath-<WWID>", so they don't match.
const multipathUUIDPrefix = "mpath-"

// IsMultipath reports whether the device at the sysfs path is a multipath device.
func IsMultipath(devicePath string) bool {
	return strings.HasPrefix(readSysFsFile(devicePath, "dm", "uuid"), multipathUUIDPrefix)
}

// MultipathUUID returns the device-mapper UUID of the multipath device of the disk with the WWID.
func MultipathUUID(wwid string) string {
	return multipathUUIDPrefix + wwid
}

// MultipathWWID returns the WWID of a disk which can be a path of a multipath device, in the form
// multipath-tools uses for it.
//
// Only a whole SCSI disk which carries a globally unique identifier qualifies: either an NAA or an
// EUI-64 designator from the Device Identification VPD page. Two devices which report the same such
// identifier are the same logical unit by the SCSI standard, so they are paths to the same disk.
// Any weaker identifier (e.g. the T10 vendor identifier made up from the vendor, the model and the
// serial number, which every virtual disk of a hypervisor might share) is not trusted.
//
// The WWID is what scsi_id reports, and what multipath-tools names the multipath device after: the
// designator in hex, prefixed by its type ("3" for NAA, "2" for EUI-64), while the kernel reports
// the same designator as "naa.<hex>" or "eui.<hex>".
func MultipathWWID(devicePath string) (string, bool) {
	// the device type 0 is a direct-access block device, as opposed to a CD-ROM, a tape, an enclosure, etc.
	if readSysFsFile(devicePath, "device", "type") != "0" {
		return "", false
	}

	subsystem, err := filepath.EvalSymlinks(filepath.Join(devicePath, "device", "subsystem"))
	if err != nil || filepath.Base(subsystem) != "scsi" {
		return "", false
	}

	designator, id, ok := strings.Cut(readSysFsFile(devicePath, "device", "wwid"), ".")
	if !ok || id == "" || strings.Trim(id, "0") == "" {
		return "", false
	}

	switch designator {
	case "naa":
		return "3" + strings.ToLower(id), true
	case "eui":
		return "2" + strings.ToLower(id), true
	default:
		return "", false
	}
}

// MultipathGroups returns the disks which can be paths of a multipath device, grouped by their WWID.
//
// The paths of each group are sorted. A disk which is held by any device other than a multipath
// device (e.g. it is a physical volume of an active LVM volume group) is in use as is, so it is left alone.
func MultipathGroups(devices iter.Seq[*block.Device]) map[string][]string {
	groups := map[string][]string{}

	for device := range devices {
		if device.TypedSpec().Type != block.DeviceTypeDisk {
			continue
		}

		wwid, ok := MultipathWWID(device.TypedSpec().DevicePath)
		if !ok {
			continue
		}

		if heldByOther(device.TypedSpec().DevicePath, wwid) {
			continue
		}

		groups[wwid] = append(groups[wwid], device.Metadata().ID())
	}

	for _, paths := range groups {
		slices.Sort(paths)
	}

	return groups
}

// heldByOther reports whether the disk is held by a device other than its own multipath device.
func heldByOther(devicePath, wwid string) bool {
	holders, err := os.ReadDir(filepath.Join(devicePath, "holders"))
	if err != nil {
		return false
	}

	for _, holder := range holders {
		if readSysFsFile(devicePath, "holders", holder.Name(), "dm", "uuid") != MultipathUUID(wwid) {
			return true
		}
	}

	return false
}

// MultipathPaths returns the devices which are paths of a multipath device, mapped to the ID of
// the multipath device they belong to.
//
// Each path of a multipath device is a separate device which reads the same data as the multipath
// device itself, so it announces the very same partition table and filesystems. Once the multipath
// device is assembled, the paths are only a way to reach it, and nothing should be read from or
// written to them directly. The partitions of a path are paths as well, as they are a view into the
// same data.
//
// If claim is set (Talos assembles multipath devices itself), every disk which can be a path (see
// MultipathGroups) is a path right away, even before its multipath device is assembled: otherwise
// the volumes on it could be discovered and put to use in the meantime, and a disk which is in use
// can't be made a path of a multipath device anymore. Such a path is mapped to the UUID of the
// multipath device it is going to belong to.
func MultipathPaths(devices iter.Seq[*block.Device], claim bool) map[string]string {
	paths := map[string]string{}

	var partitions []*block.Device

	for device := range devices {
		if device.TypedSpec().Type == block.DeviceTypePartition {
			partitions = append(partitions, device)

			continue
		}

		if len(device.TypedSpec().Secondaries) == 0 || !IsMultipath(device.TypedSpec().DevicePath) {
			continue
		}

		for _, secondary := range device.TypedSpec().Secondaries {
			paths[secondary] = device.Metadata().ID()
		}
	}

	if claim {
		claimPaths(devices, paths)
	}

	for _, partition := range partitions {
		if multipath, ok := paths[partition.TypedSpec().Parent]; ok {
			paths[partition.Metadata().ID()] = multipath
		}
	}

	return paths
}

// claimPaths adds every disk which can be a path, and is not a path of an assembled multipath
// device yet, mapped to the UUID of the multipath device it is going to belong to.
func claimPaths(devices iter.Seq[*block.Device], paths map[string]string) {
	for wwid, group := range MultipathGroups(devices) {
		for _, path := range group {
			if _, assembled := paths[path]; !assembled {
				paths[path] = MultipathUUID(wwid)
			}
		}
	}
}

// DeviceMapperUUID returns the UUID of the device-mapper device at the sysfs path, and empty for any other device.
func DeviceMapperUUID(devicePath string) string {
	return readSysFsFile(devicePath, "dm", "uuid")
}

// Sectors returns the size of the device at the sysfs path in 512-byte sectors, which is how the kernel
// reports the size of every block device regardless of its logical block size.
func Sectors(devicePath string) uint64 {
	sectors, err := strconv.ParseUint(readSysFsFile(devicePath, "size"), 10, 64)
	if err != nil {
		return 0
	}

	return sectors
}

// SCSIDeviceRunning reports whether the SCSI device of the disk at the sysfs path accepts commands.
//
// The device is "running" while it can be reached. A path which is lost goes "blocked" while the
// transport tries to recover it, and "transport-offline" or "offline" once it gives up.
func SCSIDeviceRunning(devicePath string) bool {
	return readSysFsFile(devicePath, "device", "state") == "running"
}

// FindDeviceMapper returns the kernel name ("dm-N") and the sorted secondaries of the device-mapper
// device with the UUID, looking it up in the sysfs block directory at root (e.g. /sys/block).
//
// It reads the state of the kernel right now, which a device-mapper device which has just been
// created or reloaded might not have been announced with an event yet.
func FindDeviceMapper(root, uuid string) (string, []string, bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", nil, false
	}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "dm-") {
			continue
		}

		devicePath := filepath.Join(root, entry.Name())

		if readSysFsFile(devicePath, "dm", "uuid") != uuid {
			continue
		}

		secondaries := ReadSecondaries(devicePath)
		slices.Sort(secondaries)

		return entry.Name(), secondaries, true
	}

	return "", nil, false
}
