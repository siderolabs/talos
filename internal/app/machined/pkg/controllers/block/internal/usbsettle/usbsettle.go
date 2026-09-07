// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package usbsettle implements waiting for the USB bus to be enumerated.
package usbsettle

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
	DefaultPollInterval = 250 * time.Millisecond
	DefaultQuietPeriod  = time.Second
	DefaultTimeout      = 15 * time.Second
)

// pciClassUSBHost is the PCI class/subclass of a USB host controller (0x0c = serial bus, 0x03 = USB).
const pciClassUSBHost = 0x0c03

// pciProgIfUSBDevice is the PCI prog-if of a USB *device* (gadget) controller, which has no bus to enumerate.
const pciProgIfUSBDevice = 0xfe

// usbClassMassStorage is the USB interface class of a mass storage device.
const usbClassMassStorage = 0x08

// Settler waits for the USB bus to be enumerated.
//
// USB enumeration is asynchronous to udev event processing: the host controller, the hub and the
// storage drivers are separate kernel modules, and each one is loaded from a udev event triggered by
// the device the previous one has created. `udevadm settle` only waits for the events which are
// already queued, so it happily returns while the USB bus scan is still in flight, and a USB-attached
// system disk shows up seconds later - long after Talos has concluded that it is not installed.
//
// The Settler bridges that gap by waiting until:
//
//   - every PCI USB host controller has a driver bound to it (the HCD module is loaded, and with it the
//     root hubs are registered);
//   - every USB mass storage interface has a driver bound to it and a block device underneath (this
//     also covers the `delay_use` SCSI scan delay of usb-storage);
//   - the set of USB devices stays unchanged for QuietPeriod, so that devices which are still being
//     enumerated are given a chance to appear.
//
// If the conditions are not met within Timeout, the Settler gives up and lets the caller proceed:
// a USB device which never enumerates should not hold the boot forever.
type Settler struct {
	// SysfsRoot is the sysfs mount point, defaults to "/sys".
	SysfsRoot string

	// PollInterval, QuietPeriod and Timeout default to the Default* constants.
	PollInterval time.Duration
	QuietPeriod  time.Duration
	Timeout      time.Duration
}

// Wait waits for the USB bus to settle down (see Settler).
//
//nolint:gocyclo
func (s *Settler) Wait(ctx context.Context, logger *zap.Logger) error {
	controllers, err := s.usbHostControllers()
	if err != nil {
		return fmt.Errorf("error scanning PCI USB host controllers: %w", err)
	}

	devices, err := s.usbDevices()
	if err != nil {
		return fmt.Errorf("error scanning USB devices: %w", err)
	}

	// no USB host controller and no USB device (already enumerated by a non-PCI controller):
	// there is nothing to wait for, don't delay the boot
	if len(controllers) == 0 && len(devices) == 0 {
		logger.Debug("no USB host controllers found, skipping USB settle")

		return nil
	}

	var (
		start      = time.Now()
		deadline   = start.Add(withDefault(s.Timeout, DefaultTimeout))
		quietStart = start
	)

	ticker := time.NewTicker(withDefault(s.PollInterval, DefaultPollInterval))
	defer ticker.Stop()

	for {
		settled, pending, err := s.settled(controllers)
		if err != nil {
			return err
		}

		if settled && time.Since(quietStart) >= withDefault(s.QuietPeriod, DefaultQuietPeriod) {
			logger.Info("USB bus settled",
				zap.Duration("duration", time.Since(start)),
				zap.Int("usb_devices", len(devices)),
			)

			return nil
		}

		if time.Now().After(deadline) {
			logger.Warn("timeout waiting for the USB bus to settle",
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

		updated, err := s.usbDevices()
		if err != nil {
			return fmt.Errorf("error scanning USB devices: %w", err)
		}

		if !slices.Equal(devices, updated) {
			logger.Debug("USB devices changed", zap.Int("was", len(devices)), zap.Int("now", len(updated)))

			devices, quietStart = updated, time.Now()
		}
	}
}

// settled reports whether all the USB drivers we are waiting for are in place, and if not, what is missing.
func (s *Settler) settled(controllers []string) (bool, string, error) {
	for _, hostController := range controllers {
		bound, err := driverBound(hostController)
		if err != nil {
			return false, "", err
		}

		if !bound {
			return false, fmt.Sprintf("PCI USB host controller %s has no driver bound", filepath.Base(hostController)), nil
		}
	}

	interfaces, err := s.massStorageInterfaces()
	if err != nil {
		return false, "", err
	}

	for _, iface := range interfaces {
		bound, err := driverBound(iface)
		if err != nil {
			return false, "", err
		}

		if !bound {
			return false, fmt.Sprintf("USB mass storage interface %s has no driver bound", filepath.Base(iface)), nil
		}

		// usb-storage/uas defer the SCSI scan by `delay_use` seconds, so the block device
		// shows up well after the interface is bound
		blockDevices, err := filepath.Glob(filepath.Join(iface, "host*", "target*", "*", "block", "*"))
		if err != nil {
			return false, "", err
		}

		if len(blockDevices) == 0 {
			return false, fmt.Sprintf("USB mass storage interface %s has no block device yet", filepath.Base(iface)), nil
		}
	}

	return true, "", nil
}

// usbHostControllers returns sysfs paths of PCI USB host controllers.
//
// Non-PCI (SoC platform) controllers can't be discovered this way before their driver is bound,
// so they are only covered by the USB device set and the quiet period.
func (s *Settler) usbHostControllers() ([]string, error) {
	pciDevicesPath := filepath.Join(s.sysfsRoot(), "bus", "pci", "devices")

	entries, err := os.ReadDir(pciDevicesPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	var controllers []string

	for _, entry := range entries {
		path := filepath.Join(pciDevicesPath, entry.Name())

		class, err := readHex(filepath.Join(path, "class"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return nil, err
		}

		if (class>>8)&0xffff != pciClassUSBHost || class&0xff == pciProgIfUSBDevice {
			continue
		}

		controllers = append(controllers, path)
	}

	return controllers, nil
}

// usbDevices returns the (sorted) list of USB device and interface names, e.g. "usb1", "1-2", "1-2:1.0".
func (s *Settler) usbDevices() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.sysfsRoot(), "bus", "usb", "devices"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	slices.Sort(names)

	return names, nil
}

// massStorageInterfaces returns sysfs paths of USB interfaces of the mass storage class.
func (s *Settler) massStorageInterfaces() ([]string, error) {
	usbDevicesPath := filepath.Join(s.sysfsRoot(), "bus", "usb", "devices")

	names, err := s.usbDevices()
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
