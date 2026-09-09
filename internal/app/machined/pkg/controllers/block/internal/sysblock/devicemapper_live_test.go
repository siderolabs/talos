// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysblock_test

import (
	"fmt"
	randv2 "math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/siderolabs/go-blockdevice/v2/devicemapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// TestAugmentDeviceMapperLive runs the augmentation against sysfs as the kernel lays it out, rather
// than against a tree the test builds, so that the shape it reads stays a checked assumption.
//
// It needs no device nodes for the devices it creates: the augmentation only ever reads sysfs.
func TestAugmentDeviceMapperLive(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; must be root")
	}

	if _, err := os.Stat(devicemapper.ControlPath); err != nil {
		t.Skip("skipping test; device-mapper control device is not available")
	}

	const sectors = 2048

	control, err := devicemapper.NewControl()
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, control.Close())
	})

	// unique names, so that concurrent test runs on the same host don't collide
	suffix := fmt.Sprintf("%08x", randv2.Uint32())

	// the disk: a map which needs no backing device of its own
	disk := devicemapper.DeviceID{
		Name: "talos-test-" + suffix,
		UUID: "mpath-3600508" + suffix,
	}

	require.NoError(t, control.CreateDevice(disk, []devicemapper.Target{
		{
			Type:   "zero",
			Length: sectors,
		},
	}))

	t.Cleanup(func() {
		assert.NoError(t, control.RemoveDevice(devicemapper.ByUUID(disk.UUID)))
	})

	diskDevName := dmDevName(t, control, disk.Name)

	// a partition of it, in the shape kpartx creates one: a linear map over the disk, named after
	// it and carrying its UUID with a "part<N>-" prefix
	partition := devicemapper.DeviceID{
		Name: disk.Name + "-part1",
		UUID: "part1-" + disk.UUID,
	}

	require.NoError(t, control.CreateDevice(partition, []devicemapper.Target{
		{
			Type:   "linear",
			Params: fmt.Sprintf("%s 0", dmDevNo(t, control, disk.Name)),
			Length: sectors,
		},
	}))

	t.Cleanup(func() {
		assert.NoError(t, control.RemoveDevice(devicemapper.ByUUID(partition.UUID)))
	})

	partitionDevName := dmDevName(t, control, partition.Name)

	t.Run("partition map", func(t *testing.T) {
		values := ueventValues(partitionDevName)

		sysblock.AugmentDeviceMapper(filepath.Join("/sys/block", partitionDevName), values)

		assert.Equal(t, block.DeviceTypePartition, values["DEVTYPE"])
		assert.Equal(t, "1", values["PARTN"])
		assert.Equal(t, diskDevName, values[sysblock.ParentDeviceKey])
	})

	t.Run("the disk it is a partition of", func(t *testing.T) {
		values := ueventValues(diskDevName)

		sysblock.AugmentDeviceMapper(filepath.Join("/sys/block", diskDevName), values)

		assert.Equal(t, ueventValues(diskDevName), values)
	})
}

// ueventValues is what the kernel reports for a device-mapper device: a whole disk, always.
func ueventValues(devName string) map[string]string {
	return map[string]string{
		"DEVNAME": devName,
		"DEVTYPE": block.DeviceTypeDisk,
	}
}

func dmDevName(t *testing.T, control *devicemapper.Control, name string) string {
	t.Helper()

	devNo := dmDevNoRaw(t, control, name)

	return fmt.Sprintf("dm-%d", unix.Minor(devNo))
}

func dmDevNo(t *testing.T, control *devicemapper.Control, name string) string {
	t.Helper()

	devNo := dmDevNoRaw(t, control, name)

	return fmt.Sprintf("%d:%d", unix.Major(devNo), unix.Minor(devNo))
}

func dmDevNoRaw(t *testing.T, control *devicemapper.Control, name string) uint64 {
	t.Helper()

	devices, err := control.ListDevices()
	require.NoError(t, err)

	for _, device := range devices {
		if device.Name == name {
			return device.DevNo
		}
	}

	require.FailNow(t, "device not found", "device-mapper device %q does not exist", name)

	panic("unreachable")
}
