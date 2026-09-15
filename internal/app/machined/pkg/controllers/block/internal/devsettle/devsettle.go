// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package devsettle implements waiting for the storage devices to be enumerated.
package devsettle

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Default settings for the Settler.
const (
	DefaultPollInterval   = 250 * time.Millisecond
	DefaultQuietPeriod    = time.Second
	DefaultMMCCardTimeout = 2 * time.Second
	DefaultTimeout        = 15 * time.Second
)

// pciClassUSBHost is the PCI class/subclass of a USB host controller (0x0c = serial bus, 0x03 = USB).
const pciClassUSBHost = 0x0c03

// pciProgIfUSBDevice is the PCI prog-if of a USB *device* (gadget) controller, which has no bus to enumerate.
const pciProgIfUSBDevice = 0xfe

// usbClassMassStorage is the USB interface class of a mass storage device.
const usbClassMassStorage = 0x08

// PCI classes of storage controllers which enumerate their disks asynchronously.
const (
	pciBaseClassMassStorage = 0x01   // SCSI, IDE, RAID, SATA, SAS, NVMe, ...
	pciClassSDHost          = 0x0805 // SD host controller
	pciClassFiberChannel    = 0x0c04 // Fiber Channel
)

// pciVendorVirtio is the PCI vendor ID of virtio devices: virtio-blk is enumerated synchronously,
// and virtio-scsi registers its SCSI host synchronously (the scan is covered by the SCSI host check).
const pciVendorVirtio = 0x1af4

// Settler waits for the storage devices to be enumerated.
//
// Device enumeration is asynchronous to udev event processing: `udevadm settle` only waits for the
// uevents which are already queued, while the kernel keeps discovering devices in the background
// long after the driver has been bound:
//
//   - USB: the host controller, the hub and the storage drivers are separate kernel modules, each
//     one loaded from a udev event triggered by the device the previous one has created, and
//     usb-storage defers the SCSI scan by `delay_use` seconds;
//   - MMC/SD: card detection is a delayed work item scheduled when the host controller registers,
//     and initializing an SD card (voltage switch, tuning) takes hundreds of milliseconds;
//   - NVMe: the controller is probed asynchronously, and the namespaces are scanned by a work item
//     once the controller is live;
//   - SCSI (AHCI, SAS, RAID, virtio-scsi): the host is scanned asynchronously, and the `sd` driver
//     binds to the discovered disks asynchronously as well.
//
// So `udevadm settle` happily returns while the system disk is still being enumerated, and Talos
// concludes that it is not installed.
//
// There is no kernel interface to wait for "all the block devices" (the kernel itself doesn't know
// how many there are), so the Settler bridges the gap with a set of heuristics, waiting until:
//
//   - no kernel module is still initializing;
//   - every PCI USB host controller has a driver bound to it (the HCD module is loaded, and with it the
//     root hubs are registered);
//   - every USB mass storage interface has a driver bound to it and a block device underneath;
//   - every NVMe controller is live (or given up on);
//   - every MMC host has a card (an empty slot is given up on after MMCCardTimeout);
//   - the set of devices in sysfs stays unchanged for QuietPeriod, so that devices which are still being
//     enumerated (with no way for us to detect that) are given a chance to appear.
//
// If the conditions are not met within Timeout, the Settler gives up and lets the caller proceed:
// a device which never enumerates should not hold the boot forever.
//
// The Settler returns immediately if there are no asynchronously enumerated storage transports
// (USB, MMC, NVMe or SCSI devices, or PCI storage controllers which might still be probing) in the
// system, e.g. in a VM with virtio-blk disks only.
type Settler struct {
	// SysfsRoot is the sysfs mount point, defaults to "/sys".
	SysfsRoot string

	// PollInterval, QuietPeriod, MMCCardTimeout and Timeout default to the Default* constants.
	PollInterval   time.Duration
	QuietPeriod    time.Duration
	MMCCardTimeout time.Duration
	Timeout        time.Duration
}

// Wait waits for the storage devices to settle down (see Settler).
//
//nolint:gocyclo
func (s *Settler) Wait(ctx context.Context, logger *zap.Logger) error {
	devices, err := s.snapshot()
	if err != nil {
		return err
	}

	usbControllers, err := s.usbHostControllers()
	if err != nil {
		return fmt.Errorf("error scanning PCI USB host controllers: %w", err)
	}

	// a PCI storage controller might be probed asynchronously (e.g. NVMe), so its class device
	// (nvme, scsi_host) might not exist yet: its presence alone is a reason to wait
	storageControllers, err := s.pciStorageControllers()
	if err != nil {
		return fmt.Errorf("error scanning PCI storage controllers: %w", err)
	}

	if len(usbControllers) == 0 && len(storageControllers) == 0 && !hasAsyncTransports(devices) {
		logger.Debug("no asynchronously enumerated storage transports found, skipping device settle")

		return nil
	}

	var (
		start       = time.Now()
		deadline    = start.Add(withDefault(s.Timeout, DefaultTimeout))
		quietStart  = start
		lastPending string
		state       = &settleState{
			usbControllers: usbControllers,
			mmcHostsSeen:   map[string]time.Time{},
		}
	)

	ticker := time.NewTicker(withDefault(s.PollInterval, DefaultPollInterval))
	defer ticker.Stop()

	for {
		pending, err := s.pending(state)
		if err != nil {
			return err
		}

		// a structural condition being resolved is a change as well: the device which was being
		// waited for has just appeared, and its children (e.g. the block device) are still coming
		if pending != lastPending {
			logger.Debug("pending devices changed", zap.String("was", lastPending), zap.String("now", pending))

			lastPending, quietStart = pending, time.Now()
		}

		if pending == "" && time.Since(quietStart) >= withDefault(s.QuietPeriod, DefaultQuietPeriod) {
			logger.Info(
				"storage devices settled",
				zap.Duration("duration", time.Since(start)),
				zap.Int("devices", len(devices)),
			)

			return nil
		}

		if time.Now().After(deadline) {
			logger.Warn(
				"timeout waiting for the storage devices to settle",
				zap.Duration("duration", time.Since(start)),
				zap.String("pending", pending),
			)

			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		updated, err := s.snapshot()
		if err != nil {
			return err
		}

		if !slices.Equal(devices, updated) {
			logger.Debug("devices changed", zap.Int("was", len(devices)), zap.Int("now", len(updated)))

			devices, quietStart = updated, time.Now()
		}
	}
}

// settleState is the state carried across the polls.
type settleState struct {
	// usbControllers are sysfs paths of PCI USB host controllers, discovered once: PCI devices don't appear over time.
	usbControllers []string

	// mmcHostsSeen records when an MMC host was first seen without a card.
	mmcHostsSeen map[string]time.Time
}

// snapshot returns the (sorted) list of devices in sysfs: every device on every bus, plus the
// class devices which don't live on a bus, but which we care about (block devices, MMC hosts,
// NVMe controllers, SCSI hosts).
//
// The names are prefixed with the bus/class they come from, e.g. "bus/usb/1-2", "class/block/sda".
func (s *Settler) snapshot() ([]string, error) {
	var devices []string

	buses, err := os.ReadDir(filepath.Join(s.sysfsRoot(), "bus"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("error listing buses: %w", err)
	}

	for _, bus := range buses {
		names, err := s.readNames("bus", bus.Name(), "devices")
		if err != nil {
			return nil, err
		}

		for _, name := range names {
			devices = append(devices, "bus/"+bus.Name()+"/"+name)
		}
	}

	for _, class := range []string{"block", "mmc_host", "nvme", "scsi_host"} {
		names, err := s.readNames("class", class)
		if err != nil {
			return nil, err
		}

		for _, name := range names {
			devices = append(devices, "class/"+class+"/"+name)
		}
	}

	slices.Sort(devices)

	return devices, nil
}

// hasAsyncTransports reports whether the device snapshot contains a storage transport which is
// enumerated asynchronously to udev.
func hasAsyncTransports(devices []string) bool {
	return slices.ContainsFunc(devices, func(device string) bool {
		for _, prefix := range []string{"bus/usb/", "class/mmc_host/", "class/nvme/", "class/scsi_host/"} {
			if strings.HasPrefix(device, prefix) {
				return true
			}
		}

		return false
	})
}

// pending reports what is still being enumerated (as a human-readable string), or "" if all
// structural conditions are met.
func (s *Settler) pending(state *settleState) (string, error) {
	for _, check := range []func(*settleState) (string, error){
		s.pendingModules,
		s.pendingUSBHostControllers,
		s.pendingUSBMassStorage,
		s.pendingNVMe,
		s.pendingMMC,
	} {
		pending, err := check(state)
		if err != nil || pending != "" {
			return pending, err
		}
	}

	return "", nil
}

// pendingModules reports a kernel module which is still running its init function: the driver
// probe is usually synchronous with it (e.g. a RAID controller initializing its firmware).
func (s *Settler) pendingModules(*settleState) (string, error) {
	names, err := s.readNames("module")
	if err != nil {
		return "", err
	}

	for _, name := range names {
		contents, err := os.ReadFile(filepath.Join(s.sysfsRoot(), "module", name, "initstate"))
		if err != nil {
			// built-in modules don't have initstate
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return "", err
		}

		if strings.TrimSpace(string(contents)) == "coming" {
			return fmt.Sprintf("kernel module %s is still initializing", name), nil
		}
	}

	return "", nil
}

// pendingUSBHostControllers reports a PCI USB host controller which has no driver bound yet.
func (s *Settler) pendingUSBHostControllers(state *settleState) (string, error) {
	for _, hostController := range state.usbControllers {
		bound, err := driverBound(hostController)
		if err != nil {
			return "", err
		}

		if !bound {
			return fmt.Sprintf("PCI USB host controller %s has no driver bound", filepath.Base(hostController)), nil
		}
	}

	return "", nil
}

// pendingUSBMassStorage reports a USB mass storage interface which has no driver bound or no block device yet.
func (s *Settler) pendingUSBMassStorage(*settleState) (string, error) {
	interfaces, err := s.massStorageInterfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		bound, err := driverBound(iface)
		if err != nil {
			return "", err
		}

		if !bound {
			return fmt.Sprintf("USB mass storage interface %s has no driver bound", filepath.Base(iface)), nil
		}

		// usb-storage/uas defer the SCSI scan by `delay_use` seconds, so the block device
		// shows up well after the interface is bound
		blockDevices, err := filepath.Glob(filepath.Join(iface, "host*", "target*", "*", "block", "*"))
		if err != nil {
			return "", err
		}

		if len(blockDevices) == 0 {
			return fmt.Sprintf("USB mass storage interface %s has no block device yet", filepath.Base(iface)), nil
		}
	}

	return "", nil
}

// pendingNVMe reports an NVMe controller which is still being initialized (or reset).
//
// Controllers which are dead or being deleted are not waited for.
func (s *Settler) pendingNVMe(*settleState) (string, error) {
	names, err := s.readNames("class", "nvme")
	if err != nil {
		return "", err
	}

	for _, name := range names {
		contents, err := os.ReadFile(filepath.Join(s.sysfsRoot(), "class", "nvme", name, "state"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return "", err
		}

		switch state := strings.TrimSpace(string(contents)); state {
		case "new", "connecting", "resetting":
			return fmt.Sprintf("NVMe controller %s is %s", name, state), nil
		}
	}

	return "", nil
}

// pendingMMC reports an MMC host which has no card detected yet.
//
// There is no way to tell an empty slot from a card which is still being initialized, so an
// empty host is only waited for MMCCardTimeout since it was first seen: an empty slot is detected
// within milliseconds, while an SD card takes up to several hundred milliseconds to initialize.
func (s *Settler) pendingMMC(state *settleState) (string, error) {
	names, err := s.readNames("class", "mmc_host")
	if err != nil {
		return "", err
	}

	var pending string

	for _, name := range names {
		// the card shows up as a child device named "<host>:<rca>", e.g. "mmc0:aaaa"
		cards, err := filepath.Glob(filepath.Join(s.sysfsRoot(), "class", "mmc_host", name, name+":*"))
		if err != nil {
			return "", err
		}

		if len(cards) > 0 {
			continue
		}

		// keep going after the first pending host, so that the timeouts of all the empty hosts
		// run concurrently, not one after another
		firstSeen, ok := state.mmcHostsSeen[name]
		if !ok {
			firstSeen = time.Now()
			state.mmcHostsSeen[name] = firstSeen
		}

		if pending == "" && time.Since(firstSeen) < withDefault(s.MMCCardTimeout, DefaultMMCCardTimeout) {
			pending = fmt.Sprintf("MMC host %s has no card yet", name)
		}
	}

	return pending, nil
}

// usbHostControllers returns sysfs paths of PCI USB host controllers.
//
// Non-PCI (SoC platform) controllers can't be discovered this way before their driver is bound,
// so they are only covered by the USB device set and the quiet period.
func (s *Settler) usbHostControllers() ([]string, error) {
	return s.pciDevices(func(class, _ uint64) bool {
		return (class>>8)&0xffff == pciClassUSBHost && class&0xff != pciProgIfUSBDevice
	})
}

// pciStorageControllers returns sysfs paths of PCI storage controllers (other than USB), which
// enumerate their disks asynchronously to the driver probe.
//
// Non-PCI (SoC platform) controllers can't be discovered this way, so they are only covered by
// the class devices they register (mmc_host, scsi_host) and the quiet period.
func (s *Settler) pciStorageControllers() ([]string, error) {
	return s.pciDevices(func(class, vendor uint64) bool {
		switch {
		case class>>16 == pciBaseClassMassStorage:
			return vendor != pciVendorVirtio
		case (class>>8)&0xffff == pciClassSDHost, (class>>8)&0xffff == pciClassFiberChannel:
			return true
		default:
			return false
		}
	})
}

// pciDevices returns sysfs paths of PCI devices matching the class (and vendor) predicate.
func (s *Settler) pciDevices(match func(class, vendor uint64) bool) ([]string, error) {
	pciDevicesPath := filepath.Join(s.sysfsRoot(), "bus", "pci", "devices")

	names, err := s.readNames("bus", "pci", "devices")
	if err != nil {
		return nil, err
	}

	var devices []string

	for _, name := range names {
		path := filepath.Join(pciDevicesPath, name)

		class, err := readHex(filepath.Join(path, "class"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return nil, err
		}

		vendor, err := readHex(filepath.Join(path, "vendor"))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}

		if !match(class, vendor) {
			continue
		}

		devices = append(devices, path)
	}

	return devices, nil
}

// massStorageInterfaces returns sysfs paths of USB interfaces of the mass storage class.
func (s *Settler) massStorageInterfaces() ([]string, error) {
	usbDevicesPath := filepath.Join(s.sysfsRoot(), "bus", "usb", "devices")

	names, err := s.readNames("bus", "usb", "devices")
	if err != nil {
		return nil, err
	}

	var interfaces []string

	for _, name := range names {
		// USB interfaces are named "<device>:<config>.<interface>", USB devices don't have the ':' part
		if !strings.Contains(name, ":") {
			continue
		}

		path := filepath.Join(usbDevicesPath, name)

		class, err := readHex(filepath.Join(path, "bInterfaceClass"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return nil, err
		}

		if class != usbClassMassStorage {
			continue
		}

		interfaces = append(interfaces, path)
	}

	return interfaces, nil
}

// readNames returns the (sorted) entry names of a sysfs directory, or nil if the directory doesn't exist.
func (s *Settler) readNames(elem ...string) ([]string, error) {
	path := filepath.Join(append([]string{s.sysfsRoot()}, elem...)...)

	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("error listing %q: %w", path, err)
	}

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	slices.Sort(names)

	return names, nil
}

// driverBound reports whether the kernel has bound a driver to the device.
func driverBound(devicePath string) (bool, error) {
	_, err := os.Stat(filepath.Join(devicePath, "driver"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *Settler) sysfsRoot() string {
	if s.SysfsRoot != "" {
		return s.SysfsRoot
	}

	return "/sys"
}

// readHex reads a sysfs attribute holding a hex number, with or without the "0x" prefix.
func readHex(path string) (uint64, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	value := strings.TrimPrefix(strings.TrimSpace(string(contents)), "0x")

	parsed, err := strconv.ParseUint(value, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing %q: %w", path, err)
	}

	return parsed, nil
}

func withDefault(value, defaultValue time.Duration) time.Duration {
	if value <= 0 {
		return defaultValue
	}

	return value
}
