// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysblock_test

import (
	"maps"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestMultipathPaths(t *testing.T) {
	t.Parallel()

	root := buildSysFs(t, map[string]string{
		// the multipath device over sda and sdb, and its partition map
		"dm-0/dm/uuid": mpathUUID + "\n",
		"dm-1/dm/uuid": "part1-" + mpathUUID + "\n",
		// an LVM logical volume over sdc
		"dm-2/dm/uuid": "LVM-AbCdEf0001000200030004000500060007-lvname\n",
		// a multipath device which has no paths (yet)
		"dm-3/dm/uuid": "mpath-36005076810800567c8000000000009f8\n",
	}, nil)

	disk := func(id string, secondaries ...string) *block.Device {
		dev := block.NewDevice(block.NamespaceName, id)
		dev.TypedSpec().Type = block.DeviceTypeDisk
		dev.TypedSpec().DevicePath = filepath.Join(root, id)
		dev.TypedSpec().Secondaries = secondaries

		return dev
	}

	partition := func(id, parent string) *block.Device {
		dev := block.NewDevice(block.NamespaceName, id)
		dev.TypedSpec().Type = block.DeviceTypePartition
		dev.TypedSpec().DevicePath = filepath.Join(root, parent, id)
		dev.TypedSpec().Parent = parent

		return dev
	}

	devices := []*block.Device{
		// partitions come first on purpose: they are resolved regardless of the order
		partition("sda1", "sda"),
		partition("sdb1", "sdb"),
		partition("sdc1", "sdc"),
		partition("dm-1", "dm-0"),
		disk("sda"),
		disk("sdb"),
		disk("sdc"),
		disk("sdd"),
		disk("dm-0", "sda", "sdb"),
		disk("dm-2", "sdc1"),
		disk("dm-3"),
	}

	assert.Equal(t, map[string]string{
		"sda":  "dm-0",
		"sda1": "dm-0",
		"sdb":  "dm-0",
		"sdb1": "dm-0",
	}, sysblock.MultipathPaths(slices.Values(devices), false))
}

func TestIsMultipath(t *testing.T) {
	t.Parallel()

	root := buildSysFs(t, map[string]string{
		"dm-0/dm/uuid": mpathUUID + "\n",
		"dm-1/dm/uuid": "part1-" + mpathUUID + "\n",
		"dm-2/dm/uuid": "CRYPT-LUKS2-abcdef1234567890abcdef1234567890-cryptname\n",
		"sda/size":     "3907029168\n",
	}, nil)

	assert.True(t, sysblock.IsMultipath(filepath.Join(root, "dm-0")))
	assert.False(t, sysblock.IsMultipath(filepath.Join(root, "dm-1")), "a partition map is not a multipath device")
	assert.False(t, sysblock.IsMultipath(filepath.Join(root, "dm-2")))
	assert.False(t, sysblock.IsMultipath(filepath.Join(root, "sda")))
	assert.False(t, sysblock.IsMultipath(filepath.Join(root, "missing")))
}

// scsiDisk returns the sysfs files of a whole SCSI disk with the WWID, as the kernel reports it.
func scsiDisk(id, wwid string) map[string]string {
	return map[string]string{
		id + "/device/type":  "0\n",
		id + "/device/wwid":  wwid + "\n",
		id + "/device/state": "running\n",
		id + "/size":         "2048\n",
		"bus/scsi/.keep":     "",
	}
}

// scsiLinks returns the sysfs links of a SCSI disk.
func scsiLinks(id string) map[string]string {
	return map[string]string{
		id + "/device/subsystem": "../../bus/scsi",
	}
}

func merge(ms ...map[string]string) map[string]string {
	result := map[string]string{}

	for _, m := range ms {
		maps.Copy(result, m)
	}

	return result
}

func TestMultipathWWID(t *testing.T) {
	t.Parallel()

	root := buildSysFs(t,
		merge(
			scsiDisk("naa", "naa.600508B4000156D70001200000B0000"),
			scsiDisk("eui", "eui.0025385b71b07e2f"),
			scsiDisk("t10", "t10.ATA     QEMU HARDDISK                           QM00001"),
			scsiDisk("zero", "naa.00000000000000000000000000000000"),
			scsiDisk("iqn", "iqn.2003-01.org.linux-iscsi.target,t,0x1"),
			scsiDisk("cdrom", "naa.5000c500a1b2c3d4"),
			map[string]string{
				"cdrom/device/type": "5\n",
				// an NVMe namespace reports a WWID too, but NVMe multipath is done by the kernel
				"nvme0n1/wwid": "eui.0025385b71b07e2f\n",
			},
		),
		merge(scsiLinks("naa"), scsiLinks("eui"), scsiLinks("t10"), scsiLinks("zero"), scsiLinks("iqn"), scsiLinks("cdrom")),
	)

	for _, test := range []struct {
		device string
		wwid   string
	}{
		{device: "naa", wwid: "3600508b4000156d70001200000b0000"},
		{device: "eui", wwid: "20025385b71b07e2f"},
		{device: "t10"},
		{device: "zero"},
		{device: "iqn"},
		{device: "cdrom"},
		{device: "nvme0n1"},
		{device: "missing"},
	} {
		t.Run(test.device, func(t *testing.T) {
			t.Parallel()

			wwid, ok := sysblock.MultipathWWID(filepath.Join(root, test.device))

			assert.Equal(t, test.wwid != "", ok)
			assert.Equal(t, test.wwid, wwid)
		})
	}
}

func TestMultipathGroupsAndClaim(t *testing.T) {
	t.Parallel()

	const (
		lun1 = "naa.5000c500a1b2c3d4"
		lun2 = "naa.5000c500a1b2c3d5"
	)

	root := buildSysFs(t,
		merge(
			// two paths to LUN 1, one of them a path of its multipath device already
			scsiDisk("sda", lun1),
			scsiDisk("sdb", lun1),
			// a single path to LUN 2
			scsiDisk("sdc", lun2),
			// a disk with no unique identifier
			scsiDisk("sdd", "t10.ATA     QEMU HARDDISK                           QM00001"),
			// a disk which is in use by LVM, it is left alone
			scsiDisk("sde", "naa.5000c500a1b2c3d6"),
			map[string]string{
				"dm-0/dm/uuid": "mpath-35000c500a1b2c3d4\n",
				"dm-1/dm/uuid": "LVM-AbCdEf0001000200030004000500060007-lvname\n",
			},
		),
		merge(
			scsiLinks("sda"), scsiLinks("sdb"), scsiLinks("sdc"), scsiLinks("sdd"), scsiLinks("sde"),
			map[string]string{
				"sda/holders/dm-0": "../../dm-0",
				"sde/holders/dm-1": "../../dm-1",
			},
		),
	)

	disk := func(id string, secondaries ...string) *block.Device {
		dev := block.NewDevice(block.NamespaceName, id)
		dev.TypedSpec().Type = block.DeviceTypeDisk
		dev.TypedSpec().DevicePath = filepath.Join(root, id)
		dev.TypedSpec().Secondaries = secondaries

		return dev
	}

	partition := func(id, parent string) *block.Device {
		dev := block.NewDevice(block.NamespaceName, id)
		dev.TypedSpec().Type = block.DeviceTypePartition
		dev.TypedSpec().DevicePath = filepath.Join(root, parent, id)
		dev.TypedSpec().Parent = parent

		return dev
	}

	devices := []*block.Device{
		disk("sdb"),
		disk("sda"),
		disk("sdc"),
		disk("sdd"),
		disk("sde"),
		partition("sdc1", "sdc"),
		partition("sdd1", "sdd"),
		disk("dm-0", "sda"),
		disk("dm-1", "sde"),
	}

	assert.Equal(t, map[string][]string{
		"35000c500a1b2c3d4": {"sda", "sdb"},
		"35000c500a1b2c3d5": {"sdc"},
	}, sysblock.MultipathGroups(slices.Values(devices)))

	t.Run("without claim", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]string{
			"sda": "dm-0",
		}, sysblock.MultipathPaths(slices.Values(devices), false))
	})

	t.Run("with claim", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]string{
			"sda":  "dm-0",
			"sdb":  "mpath-35000c500a1b2c3d4",
			"sdc":  "mpath-35000c500a1b2c3d5",
			"sdc1": "mpath-35000c500a1b2c3d5",
		}, sysblock.MultipathPaths(slices.Values(devices), true))
	})
}

func TestFindDeviceMapper(t *testing.T) {
	t.Parallel()

	root := buildSysFs(t, map[string]string{
		"dm-0/dm/uuid": "LVM-AbCdEf0001000200030004000500060007-lvname\n",
		"dm-3/dm/uuid": mpathUUID + "\n",
		"sda/size":     "2048\n",
	}, map[string]string{
		"dm-3/slaves/sdc": "../../sdc",
		"dm-3/slaves/sda": "../../sda",
	})

	name, secondaries, ok := sysblock.FindDeviceMapper(root, mpathUUID)
	assert.True(t, ok)
	assert.Equal(t, "dm-3", name)
	assert.Equal(t, []string{"sda", "sdc"}, secondaries)

	_, _, ok = sysblock.FindDeviceMapper(root, "mpath-36005076810800567c8000000000009f8")
	assert.False(t, ok)
}
