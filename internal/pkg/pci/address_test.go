// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pci_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/pci"
)

func TestAddressFromBusPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		busPath  string
		expected string
	}{
		{busPath: "/pci0000:80/0000:80:05.1/0000:85:00.0/nvme", expected: "0000:85:00.0"},
		{busPath: "/pci0000:00/0000:00:1f.2/ata1/host0/target0:0:0/0:0:0:0", expected: "0000:00:1f.2"},
		{busPath: "/pci0000:00/0000:00:01.0/0000:01:00.0/0000:02:03.0/0000:05:00.1/nvme", expected: "0000:05:00.1"},
		{busPath: "/virtual", expected: ""},
		{busPath: "", expected: ""},
	} {
		t.Run(test.busPath, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, pci.AddressFromBusPath(test.busPath))
		})
	}
}

func TestDiskAddresses(t *testing.T) {
	t.Parallel()

	sysfs := t.TempDir()

	// A multipath namespace head reached through two controllers, as laid out by the kernel.
	head := filepath.Join(sysfs, "devices", "virtual", "nvme-subsystem", "nvme-subsys0", "nvme0n1")
	require.NoError(t, os.MkdirAll(filepath.Join(head, "multipath"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "class", "block"), 0o755))
	require.NoError(t, os.Symlink(head, filepath.Join(sysfs, "class", "block", "nvme0n1")))

	for _, path := range []struct{ name, pci string }{
		{name: "nvme0c0n1", pci: "0000:41:00.0"},
		{name: "nvme0c1n1", pci: "0000:42:00.0"},
	} {
		target := filepath.Join(sysfs, "devices", "pci0000:40", "0000:40:01.1", path.pci, "nvme", "nvme0", path.name)
		require.NoError(t, os.MkdirAll(target, 0o755))
		require.NoError(t, os.Symlink(target, filepath.Join(head, "multipath", path.name)))
	}

	assert.Equal(t, []string{"0000:41:00.0", "0000:42:00.0"}, pci.DiskAddresses(sysfs, "nvme0n1", "/virtual/nvme-subsystem"))
	assert.Equal(t, []string{"0000:85:00.0"}, pci.DiskAddresses(sysfs, "nvme6n1", "/pci0000:80/0000:80:05.1/0000:85:00.0/nvme"))
	assert.Empty(t, pci.DiskAddresses(sysfs, "loop4", "/virtual"))
}

// sysfsBlock lays out a block device in a fake sysfs at the given device path, linked from
// class/block, and returns its device directory.
func sysfsBlock(t *testing.T, sysfs, devicePath string) string {
	t.Helper()

	dir := filepath.Join(sysfs, "devices", devicePath)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "class", "block"), 0o755))
	require.NoError(t, os.Symlink(dir, filepath.Join(sysfs, "class", "block", filepath.Base(dir))))

	return dir
}

func TestBlockDeviceAddresses(t *testing.T) {
	t.Parallel()

	sysfs := t.TempDir()

	disk := sysfsBlock(t, sysfs, "pci0000:40/0000:40:01.1/0000:41:00.0/nvme/nvme0/nvme0n1")
	partition := filepath.Join(disk, "nvme0n1p1")
	require.NoError(t, os.MkdirAll(partition, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(partition, "partition"), []byte("1\n"), 0o644))
	require.NoError(t, os.Symlink(partition, filepath.Join(sysfs, "class", "block", "nvme0n1p1")))

	// A multipath head reached through a controller on 0000:42:00.0, and one with no path yet.
	head := sysfsBlock(t, sysfs, "virtual/nvme-subsystem/nvme-subsys1/nvme1n1")
	path := filepath.Join(sysfs, "devices", "pci0000:40", "0000:40:01.2", "0000:42:00.0", "nvme", "nvme1", "nvme1c1n1")
	require.NoError(t, os.MkdirAll(path, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(head, "multipath"), 0o755))
	require.NoError(t, os.Symlink(path, filepath.Join(head, "multipath", "nvme1c1n1")))

	pathless := sysfsBlock(t, sysfs, "virtual/nvme-subsystem/nvme-subsys2/nvme2n1")
	require.NoError(t, os.MkdirAll(filepath.Join(pathless, "multipath"), 0o755))

	// A head whose controllers all failed to enumerate, without even a multipath directory.
	sysfsBlock(t, sysfs, "virtual/nvme-subsystem/nvme-subsys3/nvme3n1")

	// A device-mapper device on the partition, and one not set up yet.
	dm := sysfsBlock(t, sysfs, "virtual/block/dm-0")
	require.NoError(t, os.MkdirAll(filepath.Join(dm, "dm"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dm, "slaves"), 0o755))
	require.NoError(t, os.Symlink(partition, filepath.Join(dm, "slaves", "nvme0n1p1")))

	empty := sysfsBlock(t, sysfs, "virtual/block/dm-1")
	require.NoError(t, os.MkdirAll(filepath.Join(empty, "dm"), 0o755))

	sysfsBlock(t, sysfs, "virtual/block/loop0")

	for _, test := range []struct {
		name string

		expected []string
		known    bool
	}{
		{name: "nvme0n1", expected: []string{"0000:41:00.0"}, known: true},
		{name: "nvme0n1p1", expected: []string{"0000:41:00.0"}, known: true},
		{name: "nvme1n1", expected: []string{"0000:42:00.0"}, known: true},
		{name: "nvme2n1"},
		{name: "nvme3n1"},
		{name: "dm-0", expected: []string{"0000:41:00.0"}, known: true},
		{name: "dm-1"},
		{name: "loop0", known: true},
		{name: "sdz"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			addresses, known := pci.BlockDeviceAddresses(sysfs, test.name)

			assert.Equal(t, test.expected, addresses)
			assert.Equal(t, test.known, known)
		})
	}
}
