// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// procCountingDevices counts the descriptors read under /proc, and knows no device file.
type procCountingDevices struct {
	procfs string
	reads  int
}

func (d *procCountingDevices) DeviceNumber(path string) (deviceNumber, bool, error) {
	if strings.HasPrefix(path, d.procfs) {
		d.reads++
	}

	return deviceNumber{}, false, nil
}

// What processes hold open is read from /proc once per reconcile, however many devices are checked:
// a busy host has a great many descriptors.
func TestOpenFilesReadOncePerReconcile(t *testing.T) {
	t.Parallel()

	sysfs, procfs := t.TempDir(), t.TempDir()

	for id, group := range map[string]string{"0000:01:00.0": "7", "0000:02:00.0": "8"} {
		device := filepath.Join(sysfs, "bus", "pci", "devices", id)
		require.NoError(t, os.MkdirAll(device, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "kernel", "iommu_groups", group), 0o755))
		require.NoError(t, os.Symlink(filepath.Join(sysfs, "kernel", "iommu_groups", group), filepath.Join(device, "iommu_group")))
	}

	require.NoError(t, os.MkdirAll(filepath.Join(procfs, "4242", "fd"), 0o755))
	require.NoError(t, os.Symlink("/dev/vfio/7", filepath.Join(procfs, "4242", "fd", "3")))

	devices := &procCountingDevices{procfs: procfs}
	c := &PCIDriverRebindController{SysfsPath: sysfs, ProcfsPath: procfs, devices: devices}

	assert.Contains(t, c.inUse("0000:01:00.0", "vfio-pci"), "in use by a VFIO user (pid 4242)")
	assert.Empty(t, c.inUse("0000:02:00.0", "vfio-pci"))
	assert.Equal(t, 1, devices.reads, "one walk of /proc for both devices")

	// The next reconcile reads /proc again.
	c.open = nil

	assert.Contains(t, c.inUse("0000:01:00.0", "vfio-pci"), "in use by a VFIO user (pid 4242)")
	assert.Equal(t, 2, devices.reads)

	// So does a device checked after another's driver changed in the same pass: unbinding and probing
	// may take a while, during which a process may open it.
	c.writeSysfs = func(string, []byte) error { return nil }

	require.NoError(t, c.unbind("0000:02:00.0"))
	assert.Empty(t, c.inUse("0000:02:00.0", "vfio-pci"))
	assert.Equal(t, 3, devices.reads, "read again after an unbind")

	require.NoError(t, c.probe("0000:02:00.0"))
	assert.Empty(t, c.inUse("0000:02:00.0", "vfio-pci"))
	assert.Equal(t, 4, devices.reads, "read again after a probe")
}
