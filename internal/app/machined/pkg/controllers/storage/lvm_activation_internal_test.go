// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hardwarecfg "github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// fakeBlockSysfs lays out the block devices of a host with two NVMe drives, one taken from the
// host (0000:41:00.0), a SATA disk, a multipath namespace head with no path yet, and a loop device.
func fakeBlockSysfs(t *testing.T) string {
	t.Helper()

	sysfs := t.TempDir()

	block := func(devicePath string) string {
		dir := filepath.Join(sysfs, "devices", devicePath)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(sysfs, "class", "block"), 0o755))
		require.NoError(t, os.Symlink(dir, filepath.Join(sysfs, "class", "block", filepath.Base(dir))))

		return dir
	}

	taken := block("pci0000:40/0000:40:01.1/0000:41:00.0/nvme/nvme0/nvme0n1")
	partition := filepath.Join(taken, "nvme0n1p1")
	require.NoError(t, os.MkdirAll(partition, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(partition, "partition"), []byte("1\n"), 0o644))
	require.NoError(t, os.Symlink(partition, filepath.Join(sysfs, "class", "block", "nvme0n1p1")))

	block("pci0000:80/0000:80:01.1/0000:85:00.0/nvme/nvme5/nvme5n1")
	block("pci0000:00/0000:00:1f.2/ata1/host0/target0:0:0/0:0:0:0/block/sda")

	head := block("virtual/nvme-subsystem/nvme-subsys2/nvme2n1")
	require.NoError(t, os.MkdirAll(filepath.Join(head, "multipath"), 0o755))

	block("virtual/block/loop0")

	return sysfs
}

// Physical volumes are placed from sysfs alone: at boot they are discovered before the Disk
// resources of their disks exist, and a volume missed then activated its whole volume group.
func TestPlaceDevice(t *testing.T) {
	t.Parallel()

	sysfs := fakeBlockSysfs(t)
	taken := map[string]struct{}{"0000:41:00.0": {}}

	for _, test := range []struct {
		name string

		expected placement
		address  string
	}{
		{name: "nvme0n1", expected: placementTaken, address: "0000:41:00.0"},
		{name: "nvme0n1p1", expected: placementTaken, address: "0000:41:00.0"},
		{name: "nvme5n1", expected: placementHost},
		{name: "sda", expected: placementHost},
		{name: "loop0", expected: placementHost},
		// A multipath head with no path yet, and a device not in sysfs yet, are not activated.
		{name: "nvme2n1", expected: placementUnknown},
		{name: "nvme9n1", expected: placementUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			placement, address := placeDevice(sysfs, test.name, taken)

			assert.Equal(t, test.expected, placement)
			assert.Equal(t, test.address, address)
		})
	}

	// Without devices taken from the host, nothing needs placing.
	placement, _ := placeDevice(sysfs, "nvme9n1", nil)
	assert.Equal(t, placementHost, placement)
}

// A volume group is only activated when none of its physical volumes is, or may be, on a device
// taken from the host.
func TestPlaceVolumeGroup(t *testing.T) {
	t.Parallel()

	sysfs := fakeBlockSysfs(t)
	taken := map[string]struct{}{"0000:41:00.0": {}}

	pvs := []lvm.PV{
		{Device: "/dev/sda", VGName: "host-vg"},
		{Device: "/dev/nvme5n1", VGName: "host-vg"},
		{Device: "/dev/sda", VGName: "guest-vg"},
		{Device: "/dev/nvme0n1p1", VGName: "guest-vg"},
		{Device: "/dev/nvme5n1", VGName: "waiting-vg"},
		{Device: "/dev/nvme2n1", VGName: "waiting-vg"},
		// A physical volume whose drive did not enumerate on this boot.
		{Device: "/dev/sda", VGName: "incomplete-vg"},
		{Device: "[unknown]", VGName: "incomplete-vg", Missing: "missing"},
	}

	for _, test := range []struct {
		vg string

		expected placement
		device   string
	}{
		{vg: "host-vg", expected: placementHost},
		{vg: "guest-vg", expected: placementTaken, device: "/dev/nvme0n1p1"},
		{vg: "waiting-vg", expected: placementUnknown, device: "/dev/nvme2n1"},
		{vg: "incomplete-vg", expected: placementUnknown, device: "[unknown]"},
	} {
		t.Run(test.vg, func(t *testing.T) {
			t.Parallel()

			placement, device := placeVolumeGroup(sysfs, test.vg, pvs, taken)

			assert.Equal(t, test.expected, placement)
			assert.Equal(t, test.device, device)
		})
	}
}

// The devices taken from the host come from the machine configuration itself: at boot, the first
// pass after it is loaded may run before the PCIDriverRebindConfig resources derived from it exist.
func TestConfigTakenAddresses(t *testing.T) {
	t.Parallel()

	rebind := hardwarecfg.NewPCIDriverRebindConfigV1Alpha1()
	rebind.MetaName = "0000:41:00.0"
	rebind.PCITargetDriver = "vfio-pci"

	cfg, err := container.New(rebind)
	require.NoError(t, err)

	assert.Equal(t, map[string]struct{}{"0000:41:00.0": {}}, configTakenAddresses(cfg))

	empty, err := container.New()
	require.NoError(t, err)

	assert.Empty(t, configTakenAddresses(empty))
}

// fakeLVM records the LVM operations activation runs. Like pvscan --vgonline, it reports a volume
// group as complete only the first time one of its physical volumes is scanned.
type fakeLVM struct {
	pvs       []lvm.PV
	online    map[string]bool
	scanned   []string
	activated []string
}

func (f *fakeLVM) PVS(context.Context) ([]lvm.PV, error) { return f.pvs, nil }

func (f *fakeLVM) PVScanAutoActivation(_ context.Context, devicePath string) (map[string]string, error) {
	f.scanned = append(f.scanned, devicePath)

	for _, pv := range f.pvs {
		if pv.Device != devicePath || pv.VGName == "" || f.online[pv.VGName] {
			continue
		}

		f.online[pv.VGName] = true

		return map[string]string{lvm.UdevKeyVGNameComplete: pv.VGName}, nil
	}

	return map[string]string{}, nil
}

func (f *fakeLVM) VGChangeActivate(_ context.Context, vgName string) error {
	f.activated = append(f.activated, vgName)

	return nil
}

// A volume group held back because one of its physical volumes cannot be placed yet is not scanned
// with --vgonline, which reports a volume group only once: it is still activated once placed.
func TestActivateHoldsBeforeVGOnline(t *testing.T) {
	t.Parallel()

	sysfs := fakeBlockSysfs(t)
	taken := map[string]struct{}{"0000:41:00.0": {}}

	fake := &fakeLVM{
		online: map[string]bool{},
		pvs: []lvm.PV{
			{Device: "/dev/sda", VGName: "host-vg"},
			// A multipath head whose path has not shown up yet.
			{Device: "/dev/nvme2n1", VGName: "host-vg"},
		},
	}

	ctrl := &LVMActivationController{
		SysfsPath:      sysfs,
		lvm:            fake,
		seenVolumes:    map[string]struct{}{},
		activatedVGs:   map[string]struct{}{},
		skippedVolumes: map[string]struct{}{},
		heldVGs:        map[string]struct{}{},
	}

	dv := block.NewDiscoveredVolume(block.NamespaceName, "sda")
	dv.TypedSpec().Name = "lvm2-pv"
	dv.TypedSpec().DevPath = "/dev/sda"

	volumes := safe.NewList[*block.DiscoveredVolume](resource.List{Items: []resource.Resource{dv}})
	logger := zaptest.NewLogger(t)

	pending, err := ctrl.activate(t.Context(), logger, volumes, taken)
	require.NoError(t, err)
	assert.True(t, pending)
	assert.Empty(t, fake.scanned, "a held volume group must not be scanned with --vgonline")
	assert.Empty(t, fake.activated)

	// The path of the multipath head shows up, on a host device.
	path := filepath.Join(sysfs, "devices", "pci0000:80", "0000:80:01.2", "0000:86:00.0", "nvme", "nvme2", "nvme2c2n1")
	require.NoError(t, os.MkdirAll(path, 0o755))
	require.NoError(t, os.Symlink(path, filepath.Join(sysfs, "devices", "virtual", "nvme-subsystem", "nvme-subsys2", "nvme2n1", "multipath", "nvme2c2n1")))

	pending, err = ctrl.activate(t.Context(), logger, volumes, taken)
	require.NoError(t, err)
	assert.False(t, pending)
	assert.Equal(t, []string{"/dev/sda"}, fake.scanned)
	assert.Equal(t, []string{"host-vg"}, fake.activated)
}

// The volume group of a physical volume is found by its device.
func TestVolumeGroupOf(t *testing.T) {
	t.Parallel()

	pvs := []lvm.PV{
		{Device: "/dev/sda", VGName: "host-vg"},
		{Device: "/dev/nvme0n1p1", VGName: "guest-vg"},
		{Device: "/dev/sdb"},
	}

	assert.Equal(t, "host-vg", volumeGroupOf(pvs, "/dev/sda"))
	assert.Equal(t, "guest-vg", volumeGroupOf(pvs, "/dev/nvme0n1p1"))
	assert.Empty(t, volumeGroupOf(pvs, "/dev/sdb"))
	assert.Empty(t, volumeGroupOf(pvs, "/dev/sdz"))
}
