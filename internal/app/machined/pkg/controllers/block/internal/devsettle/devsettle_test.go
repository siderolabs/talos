// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package devsettle_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/devsettle"
)

// fakeSysfs builds a sysfs-alike tree: /sys/bus/*/devices and /sys/class/* with entries symlinked
// to /sys/devices, the way the kernel lays it out.
type fakeSysfs struct {
	t    *testing.T
	root string
}

func newFakeSysfs(t *testing.T) *fakeSysfs {
	fs := &fakeSysfs{t: t, root: t.TempDir()}

	for _, dir := range []string{"devices", "module", "bus/pci/devices", "bus/usb/devices", "class/block"} {
		require.NoError(t, os.MkdirAll(filepath.Join(fs.root, dir), 0o755))
	}

	return fs
}

// addPCIDevice registers a PCI device with the given class, e.g. 0x0c0330 for an xHCI controller.
func (fs *fakeSysfs) addPCIDevice(addr, class string) string {
	return fs.addVendorPCIDevice(addr, "0x8086", class)
}

// addVirtioPCIDevice registers a virtio PCI device with the given class, e.g. 0x010000 for virtio-blk.
func (fs *fakeSysfs) addVirtioPCIDevice(addr, class string) string {
	return fs.addVendorPCIDevice(addr, "0x1af4", class)
}

func (fs *fakeSysfs) addVendorPCIDevice(addr, vendor, class string) string {
	path := filepath.Join(fs.root, "devices", "pci0000:00", addr)

	require.NoError(fs.t, os.MkdirAll(path, 0o755))
	fs.setAttr(path, "vendor", vendor)
	fs.setAttr(path, "class", class)
	fs.link(path, filepath.Join(fs.root, "bus/pci/devices", addr))

	return path
}

// addUSBDevice registers a USB device or interface (e.g. "usb1", "1-1", "1-1:1.0") under parent.
func (fs *fakeSysfs) addUSBDevice(parent, name string) string {
	path := filepath.Join(parent, name)

	require.NoError(fs.t, os.MkdirAll(path, 0o755))
	fs.link(path, filepath.Join(fs.root, "bus/usb/devices", name))

	return path
}

// addClassDevice registers a class device (e.g. "mmc_host"/"mmc0", "nvme"/"nvme0", "block"/"sda") under parent.
func (fs *fakeSysfs) addClassDevice(parent, class, name string) string {
	path := filepath.Join(parent, class, name)

	require.NoError(fs.t, os.MkdirAll(path, 0o755))
	require.NoError(fs.t, os.MkdirAll(filepath.Join(fs.root, "class", class), 0o755))
	fs.link(path, filepath.Join(fs.root, "class", class, name))

	return path
}

// addMMCCard registers an MMC card under the host, the way the kernel does: as a child of the host, and on the mmc bus.
func (fs *fakeSysfs) addMMCCard(hostPath, name string) string {
	path := filepath.Join(hostPath, name)

	require.NoError(fs.t, os.MkdirAll(path, 0o755))
	require.NoError(fs.t, os.MkdirAll(filepath.Join(fs.root, "bus/mmc/devices"), 0o755))
	fs.link(path, filepath.Join(fs.root, "bus/mmc/devices", name))

	return path
}

// addModule registers a kernel module with the given initstate ("coming" or "live").
func (fs *fakeSysfs) addModule(name, initstate string) {
	path := filepath.Join(fs.root, "module", name)

	require.NoError(fs.t, os.MkdirAll(path, 0o755))
	fs.setAttr(path, "initstate", initstate)
}

// setAttr sets a sysfs attribute of a device.
func (fs *fakeSysfs) setAttr(devicePath, attr, value string) {
	require.NoError(fs.t, os.WriteFile(filepath.Join(devicePath, attr), []byte(value+"\n"), 0o644))
}

// bindDriver emulates the kernel binding a driver to a device.
func (fs *fakeSysfs) bindDriver(devicePath, driver string) {
	driverPath := filepath.Join(fs.root, "bus/usb/drivers", driver)

	require.NoError(fs.t, os.MkdirAll(driverPath, 0o755))
	fs.link(driverPath, filepath.Join(devicePath, "driver"))
}

// addBlockDevice emulates the SCSI scan creating a block device for a mass storage interface.
func (fs *fakeSysfs) addBlockDevice(interfacePath, name string) {
	require.NoError(fs.t, os.MkdirAll(filepath.Join(interfacePath, "host0", "target0:0:0", "0:0:0:0", "block", name), 0o755))
}

func (fs *fakeSysfs) link(target, source string) {
	relative, err := filepath.Rel(filepath.Dir(source), target)
	require.NoError(fs.t, err)
	require.NoError(fs.t, os.Symlink(relative, source))
}

func (fs *fakeSysfs) settler() *devsettle.Settler {
	return &devsettle.Settler{
		SysfsRoot:      fs.root,
		PollInterval:   10 * time.Millisecond,
		QuietPeriod:    200 * time.Millisecond,
		MMCCardTimeout: 2 * time.Second,
		Timeout:        5 * time.Second,
	}
}

// enumerate runs f (which pokes the fake sysfs the way the kernel would) in the background, and
// makes sure it is done before the test completes.
func (fs *fakeSysfs) enumerate(f func()) {
	done := make(chan struct{})

	fs.t.Cleanup(func() { <-done })

	go func() {
		defer close(done)

		f()
	}()
}

func TestNoAsyncTransports(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	virtio := fs.addVirtioPCIDevice("0000:00:05.0", "0x010000") // virtio-blk
	fs.addClassDevice(virtio, "block", "vda")
	fs.addPCIDevice("0000:00:02.0", "0x030000") // VGA

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// nothing asynchronous in sight: no waiting, not even the quiet period
	require.Less(t, time.Since(start), 50*time.Millisecond)
}

func TestUnprobedStorageController(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	fs.addPCIDevice("0000:00:1f.2", "0x010601") // AHCI, the driver is not bound yet, so no SCSI hosts

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// a storage controller without class devices might still be probing: the quiet period applies
	require.Greater(t, time.Since(start), 200*time.Millisecond)
	require.Less(t, time.Since(start), time.Second)
}

func TestWaitForLateNVMeController(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	pci := fs.addPCIDevice("0000:01:00.0", "0x010802")

	// the NVMe driver probes asynchronously: the controller class device shows up after the settler starts
	fs.enumerate(func() {
		time.Sleep(100 * time.Millisecond)

		ctrl := fs.addClassDevice(pci, "nvme", "nvme0")
		fs.setAttr(ctrl, "state", "connecting")

		time.Sleep(300 * time.Millisecond)

		fs.setAttr(ctrl, "state", "live")
		fs.addClassDevice(ctrl, "block", "nvme0n1")
	})

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	require.Greater(t, time.Since(start), 400*time.Millisecond)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestQuietPeriod(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	ahci := fs.addPCIDevice("0000:00:1f.2", "0x010601")
	fs.addClassDevice(ahci, "scsi_host", "host0")

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// a SCSI host is scanned asynchronously, so the quiet period applies, but nothing else
	require.Greater(t, time.Since(start), 200*time.Millisecond)
	require.Less(t, time.Since(start), time.Second)
}

func TestQuietPeriodRestarts(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	ahci := fs.addPCIDevice("0000:00:1f.2", "0x010601")
	fs.addClassDevice(ahci, "scsi_host", "host0")

	// the disks show up one by one while the settler is waiting
	fs.enumerate(func() {
		time.Sleep(100 * time.Millisecond)

		fs.addClassDevice(ahci, "block", "sda")

		time.Sleep(150 * time.Millisecond)

		fs.addClassDevice(ahci, "block", "sdb")
	})

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// the quiet period restarts each time a device appears
	require.Greater(t, time.Since(start), 450*time.Millisecond)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestWaitForModuleInit(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	raid := fs.addPCIDevice("0000:03:00.0", "0x010400")
	fs.addClassDevice(raid, "scsi_host", "host0")
	fs.addModule("megaraid_sas", "coming")
	fs.addModule("sd_mod", "live")

	// the RAID controller module is initializing its firmware
	fs.enumerate(func() {
		time.Sleep(300 * time.Millisecond)

		fs.addModule("megaraid_sas", "live")
	})

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	require.Greater(t, time.Since(start), 300*time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestWaitForHostControllerDriver(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	controller := fs.addPCIDevice("0000:04:00.3", "0x0c0330")

	// the xHCI module is loaded (and the root hub registered) only after the settler starts waiting
	fs.enumerate(func() {
		time.Sleep(300 * time.Millisecond)

		fs.bindDriver(controller, "xhci_hcd")
		fs.addUSBDevice(controller, "usb1")
	})

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	require.Greater(t, time.Since(start), 300*time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestWaitForMassStorage(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	controller := fs.addPCIDevice("0000:04:00.3", "0x0c0330")
	fs.bindDriver(controller, "xhci_hcd")
	rootHub := fs.addUSBDevice(controller, "usb1")

	// the USB disk is enumerated, then usb-storage binds to it, then the SCSI scan creates the block device
	fs.enumerate(func() {
		time.Sleep(50 * time.Millisecond)

		device := fs.addUSBDevice(rootHub, "1-1")
		iface := fs.addUSBDevice(device, "1-1:1.0")
		fs.setAttr(iface, "bInterfaceClass", "08")

		time.Sleep(100 * time.Millisecond)

		fs.bindDriver(iface, "usb-storage")

		time.Sleep(300 * time.Millisecond)

		fs.addBlockDevice(iface, "sda")
	})

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// must not return before the block device shows up
	require.Greater(t, time.Since(start), 450*time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestWaitForMMCCard(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	sdhci := filepath.Join(fs.root, "devices", "platform", "1000fff000.mmc")
	host := fs.addClassDevice(sdhci, "mmc_host", "mmc0")

	// the SD card takes a while to initialize, then the block device shows up
	fs.enumerate(func() {
		time.Sleep(400 * time.Millisecond)

		card := fs.addMMCCard(host, "mmc0:aaaa")

		time.Sleep(50 * time.Millisecond)

		fs.addClassDevice(card, "block", "mmcblk0")
	})

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// must not return before the card shows up
	require.Greater(t, time.Since(start), 450*time.Millisecond)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestMMCEmptySlot(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	sdhci := filepath.Join(fs.root, "devices", "platform", "1000fff000.mmc")
	fs.addClassDevice(sdhci, "mmc_host", "mmc0")

	// no card ever shows up: the empty slot is given up on after MMCCardTimeout, not the full Timeout
	settler := fs.settler()
	settler.MMCCardTimeout = 300 * time.Millisecond

	start := time.Now()

	require.NoError(t, settler.Wait(context.Background(), zaptest.NewLogger(t)))

	require.Greater(t, time.Since(start), 300*time.Millisecond)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestMultipleEmptyMMCSlots(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)

	for i := range 4 {
		sdhci := filepath.Join(fs.root, "devices", "platform", fmt.Sprintf("%d.mmc", i))
		fs.addClassDevice(sdhci, "mmc_host", fmt.Sprintf("mmc%d", i))
	}

	// the empty slots are given up on concurrently, not one after another
	settler := fs.settler()
	settler.MMCCardTimeout = 300 * time.Millisecond

	start := time.Now()

	require.NoError(t, settler.Wait(context.Background(), zaptest.NewLogger(t)))

	require.Greater(t, time.Since(start), 300*time.Millisecond)
	require.Less(t, time.Since(start), time.Second)
}

func TestWaitForNVMe(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	pci := fs.addPCIDevice("0000:01:00.0", "0x010802")
	ctrl := fs.addClassDevice(pci, "nvme", "nvme0")
	fs.setAttr(ctrl, "state", "connecting")

	// the controller goes live, then the namespace is scanned
	fs.enumerate(func() {
		time.Sleep(300 * time.Millisecond)

		fs.setAttr(ctrl, "state", "live")

		time.Sleep(50 * time.Millisecond)

		fs.addClassDevice(ctrl, "block", "nvme0n1")
	})

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	require.Greater(t, time.Since(start), 350*time.Millisecond)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestDeadNVMe(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	pci := fs.addPCIDevice("0000:01:00.0", "0x010802")
	ctrl := fs.addClassDevice(pci, "nvme", "nvme0")
	fs.setAttr(ctrl, "state", "dead")

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// a dead controller is not waited for: only the quiet period
	require.Less(t, time.Since(start), time.Second)
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	controller := fs.addPCIDevice("0000:04:00.3", "0x0c0330")
	fs.bindDriver(controller, "xhci_hcd")
	rootHub := fs.addUSBDevice(controller, "usb1")
	device := fs.addUSBDevice(rootHub, "1-1")
	iface := fs.addUSBDevice(device, "1-1:1.0")
	fs.setAttr(iface, "bInterfaceClass", "08")

	// the block device never shows up: the settler gives up instead of holding the boot forever
	settler := fs.settler()
	settler.Timeout = 300 * time.Millisecond

	start := time.Now()

	require.NoError(t, settler.Wait(context.Background(), zaptest.NewLogger(t)))

	require.Greater(t, time.Since(start), 300*time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestIgnoresNonStorageInterfaces(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	controller := fs.addPCIDevice("0000:04:00.3", "0x0c0330")
	fs.bindDriver(controller, "xhci_hcd")
	rootHub := fs.addUSBDevice(controller, "usb1")
	device := fs.addUSBDevice(rootHub, "1-1")
	iface := fs.addUSBDevice(device, "1-1:1.0")
	fs.setAttr(iface, "bInterfaceClass", "03") // HID, no block device expected, no driver bound either

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// only the quiet period, no timeout
	require.Less(t, time.Since(start), time.Second)
}

func TestContextCanceled(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	fs.addPCIDevice("0000:04:00.3", "0x0c0330")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, fs.settler().Wait(ctx, zaptest.NewLogger(t)), context.DeadlineExceeded)
}
