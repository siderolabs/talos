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
