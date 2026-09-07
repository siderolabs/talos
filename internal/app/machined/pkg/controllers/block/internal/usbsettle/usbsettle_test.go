// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package usbsettle_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/usbsettle"
)

// fakeSysfs builds a sysfs-alike tree: /sys/bus/{pci,usb}/devices with entries symlinked
// to /sys/devices, the way the kernel lays it out.
type fakeSysfs struct {
	t    *testing.T
	root string
}

func newFakeSysfs(t *testing.T) *fakeSysfs {
	fs := &fakeSysfs{t: t, root: t.TempDir()}

	for _, dir := range []string{"devices", "bus/pci/devices", "bus/usb/devices"} {
		require.NoError(t, os.MkdirAll(filepath.Join(fs.root, dir), 0o755))
	}

	return fs
}

// addPCIDevice registers a PCI device with the given class, e.g. 0x0c0330 for an xHCI controller.
func (fs *fakeSysfs) addPCIDevice(addr, class string) string {
	path := filepath.Join(fs.root, "devices", "pci0000:00", addr)

	require.NoError(fs.t, os.MkdirAll(path, 0o755))
	require.NoError(fs.t, os.WriteFile(filepath.Join(path, "class"), []byte(class+"\n"), 0o644))
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

// bindDriver emulates the kernel binding a driver to a device.
func (fs *fakeSysfs) bindDriver(devicePath, driver string) {
	driverPath := filepath.Join(fs.root, "bus/usb/drivers", driver)

	require.NoError(fs.t, os.MkdirAll(driverPath, 0o755))
	fs.link(driverPath, filepath.Join(devicePath, "driver"))
}

// setInterfaceClass sets bInterfaceClass, "08" being the mass storage class.
func (fs *fakeSysfs) setInterfaceClass(devicePath, class string) {
	require.NoError(fs.t, os.WriteFile(filepath.Join(devicePath, "bInterfaceClass"), []byte(class+"\n"), 0o644))
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

func (fs *fakeSysfs) settler() *usbsettle.Settler {
	return &usbsettle.Settler{
		SysfsRoot:    fs.root,
		PollInterval: 10 * time.Millisecond,
		QuietPeriod:  200 * time.Millisecond,
		Timeout:      5 * time.Second,
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

func TestNoUSB(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	fs.addPCIDevice("0000:00:1f.2", "0x010601") // AHCI, not USB

	start := time.Now()

	require.NoError(t, fs.settler().Wait(context.Background(), zaptest.NewLogger(t)))

	// no USB at all: no waiting, not even the quiet period
	require.Less(t, time.Since(start), 50*time.Millisecond)
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
		fs.setInterfaceClass(iface, "08")

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

func TestTimeout(t *testing.T) {
	t.Parallel()

	fs := newFakeSysfs(t)
	controller := fs.addPCIDevice("0000:04:00.3", "0x0c0330")
	fs.bindDriver(controller, "xhci_hcd")
	rootHub := fs.addUSBDevice(controller, "usb1")
	device := fs.addUSBDevice(rootHub, "1-1")
	iface := fs.addUSBDevice(device, "1-1:1.0")
	fs.setInterfaceClass(iface, "08")

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
	fs.setInterfaceClass(iface, "03") // HID, no block device expected, no driver bound either

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
