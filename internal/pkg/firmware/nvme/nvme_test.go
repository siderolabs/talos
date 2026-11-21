// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux && (amd64 || arm64 || loong64 || riscv64 || s390x)

package nvme_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/firmware/flash"
	"github.com/siderolabs/talos/internal/pkg/firmware/guid"
	"github.com/siderolabs/talos/internal/pkg/firmware/nvme"
)

type sysfs struct {
	t    *testing.T
	root string
}

func newSysfs(t *testing.T) sysfs {
	t.Helper()

	return sysfs{t: t, root: t.TempDir()}
}

func (s sysfs) mkdir(p string) {
	require.NoError(s.t, os.MkdirAll(filepath.Join(s.root, p), 0o755))
}

func (s sysfs) write(p, content string) {
	s.mkdir(filepath.Dir(p))
	require.NoError(s.t, os.WriteFile(filepath.Join(s.root, p), []byte(content+"\n"), 0o644))
}

func (s sysfs) link(p, target string) {
	s.mkdir(filepath.Dir(p))
	require.NoError(s.t, os.Symlink(target, filepath.Join(s.root, p)))
}

// controller writes a PCIe controller with the given PCI IDs.
func (s sysfs) controller(name, model, firmware string, ids [4]string) {
	dir := "class/nvme/" + name

	s.write(dir+"/transport", "pcie")
	s.write(dir+"/model", model)
	s.write(dir+"/serial", "  S"+name+"  ")
	s.write(dir+"/firmware_rev", firmware)

	for i, attr := range []string{"vendor", "device", "subsystem_vendor", "subsystem_device"} {
		if ids[i] != "" {
			s.write(dir+"/device/"+attr, ids[i])
		}
	}
}

// fakeSysfs models a single-port drive, a dual-port multipath drive with
// two namespaces, and a controller on a non-PCIe transport.
func fakeSysfs(t *testing.T) string {
	t.Helper()

	s := newSysfs(t)

	// trailing spaces as the kernel exposes the Identify Controller strings
	s.controller("nvme0", "WD_BLACK SN850X 2000GB                  ", "620331WD", [4]string{"0x15b7", "0x5030", "0x15b7", "0x5030"})
	s.mkdir("class/nvme/nvme0/nvme0n1")
	s.mkdir("class/nvme/nvme0/ng0n1") // generic char device, not a namespace
	s.link("class/nvme-subsystem/nvme-subsys0/nvme0", "../../nvme/nvme0")

	s.controller("nvme1", "KIOXIA KCD8XVUG1T92", "1234ABCD", [4]string{"0x1e0f", "0x001b", "0x1e0f", "0x0001"})
	s.controller("nvme2", "KIOXIA KCD8XVUG1T92", "1234ABCD", [4]string{"0x1e0f", "0x001b", "0x1e0f", "0x0001"})
	s.mkdir("class/nvme/nvme1/nvme1c1n1") // multipath path devices, no block device
	s.mkdir("class/nvme/nvme1/nvme1c1n2")
	s.mkdir("class/nvme/nvme2/nvme2c2n1")
	s.mkdir("class/nvme/nvme2/nvme2c2n2")
	s.link("class/nvme-subsystem/nvme-subsys1/nvme1", "../../nvme/nvme1")
	s.link("class/nvme-subsystem/nvme-subsys1/nvme2", "../../nvme/nvme2")
	s.mkdir("class/nvme-subsystem/nvme-subsys1/nvme1n1")
	s.mkdir("class/nvme-subsystem/nvme-subsys1/nvme1n2")

	s.write("class/nvme/nvme3/transport", "tcp")
	s.write("class/nvme/nvme3/model", "Linux")

	return s.root
}

func TestList(t *testing.T) {
	t.Parallel()

	controllers, err := nvme.List(fakeSysfs(t))
	require.NoError(t, err)
	require.Len(t, controllers, 3)

	wd := controllers[0]
	assert.Equal(t, "nvme0", wd.Name)
	assert.Equal(t, "/dev/nvme0", wd.DevPath)
	assert.Equal(t, []string{"nvme0n1"}, wd.Namespaces)
	assert.Equal(t, "WD_BLACK SN850X 2000GB", wd.Model)
	assert.Equal(t, "Snvme0", wd.Serial)
	assert.Equal(t, "620331WD", wd.FirmwareVersion)
	assert.Equal(t, uint16(0x15b7), wd.VendorID)
	assert.Equal(t, uint16(0x5030), wd.DeviceID)
	assert.Equal(t, uint16(0x15b7), wd.SubsystemVendorID)
	assert.Equal(t, uint16(0x5030), wd.SubsystemDeviceID)

	assert.Contains(t, wd.InstanceIDs(), `NVME\VEN_15B7&DEV_5030`)
	assert.Contains(t, wd.InstanceIDs(), `NVME\VEN_15B7&DEV_5030&SUBSYS_15B75030`)
	assert.Equal(t, guid.GUIDs(wd.InstanceIDs()), wd.GUIDs())
	assert.Equal(t, []string{"NVME:0x15B7"}, wd.VendorIDs())

	dev := wd.Device()
	assert.Equal(t, wd.GUIDs(), dev.GUIDs)
	assert.Equal(t, flash.ProtocolNVMe, dev.Protocol)
	assert.Equal(t, "620331WD", dev.Version)
	assert.Equal(t, []string{"NVME:0x15B7"}, dev.VendorIDs)

	assert.Equal(t, flash.Target{DevPath: "/dev/nvme0"}, wd.Target())

	// multipath: both controllers share the subsystem's namespaces
	for i, name := range []string{"nvme1", "nvme2"} {
		c := controllers[1+i]
		assert.Equal(t, name, c.Name)
		assert.Equal(t, []string{"nvme1n1", "nvme1n2"}, c.Namespaces)
		assert.Equal(t, uint16(0x1e0f), c.VendorID)
		assert.Equal(t, uint16(0x001b), c.DeviceID)
		assert.Equal(t, uint16(0x0001), c.SubsystemDeviceID)
	}
}

func TestListNoNVMe(t *testing.T) {
	t.Parallel()

	controllers, err := nvme.List(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, controllers)
}

func TestListBadPCIIDs(t *testing.T) {
	t.Parallel()

	s := newSysfs(t)
	s.controller("nvme0", "X", "1", [4]string{"0x15b7", "", "0x15b7", "0x5030"})

	_, err := nvme.List(s.root)
	require.EqualError(t, err, "nvme0: missing device/device")

	s = newSysfs(t)
	s.controller("nvme0", "X", "1", [4]string{"0x15b7", "0x5030", "zz", "0x5030"})

	_, err = nvme.List(s.root)
	require.ErrorContains(t, err, `nvme0: invalid device/subsystem_vendor "zz"`)

	// a controller that can't be described doesn't hide the others
	s = newSysfs(t)
	s.controller("nvme0", "X", "1", [4]string{"0x15b7", "", "0x15b7", "0x5030"})
	s.controller("nvme1", "Y", "2", [4]string{"0x15b7", "0x5030", "0x15b7", "0x5030"})
	s.controller("nvme2", "Z", "3", [4]string{"", "0x5030", "0x15b7", "0x5030"})

	controllers, err := nvme.List(s.root)
	require.ErrorContains(t, err, "nvme0: missing device/device")
	require.ErrorContains(t, err, "nvme2: missing device/vendor")
	require.Len(t, controllers, 1)
	assert.Equal(t, "nvme1", controllers[0].Name)
}
