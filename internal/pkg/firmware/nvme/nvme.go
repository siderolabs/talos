// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux && (amd64 || arm64 || loong64 || riscv64 || s390x)

// Package nvme enumerates PCIe NVMe controllers for firmware updates.
package nvme

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/siderolabs/talos/internal/pkg/firmware/flash"
	"github.com/siderolabs/talos/internal/pkg/firmware/guid"
	"github.com/siderolabs/talos/internal/pkg/firmware/lvfs"
)

const (
	controllerClass = "class/nvme"
	subsystemClass  = "class/nvme-subsystem"
	devRoot         = "/dev"

	// transportPCIe is the only transport whose controllers are flashed.
	transportPCIe = "pcie"
)

// namespaceName matches the block devices of namespaces, e.g. nvme0n1.
var namespaceName = regexp.MustCompile(`^nvme\d+n\d+$`)

// Controller is a PCIe NVMe controller.
type Controller struct {
	// Name is the kernel name of the controller, e.g. nvme0.
	Name string
	// DevPath is the controller's character device, e.g. /dev/nvme0.
	DevPath string
	// Namespaces are the block devices of the controller's namespaces, e.g.
	// nvme0n1; with native multipath they belong to the subsystem and are
	// shared with its other controllers.
	Namespaces []string

	Model           string
	Serial          string
	FirmwareVersion string

	// PCI identifiers of the controller.
	VendorID          uint16
	DeviceID          uint16
	SubsystemVendorID uint16
	SubsystemDeviceID uint16
}

// List enumerates the PCIe NVMe controllers under a sysfs mount.
//
// A controller that can't be described, e.g. one being removed, doesn't hide
// the others: they are returned along with an error naming the failed ones.
func List(sysfs string) ([]Controller, error) {
	entries, err := os.ReadDir(filepath.Join(sysfs, controllerClass))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	var (
		controllers []Controller
		errs        []error
	)

	for _, entry := range entries {
		dir := filepath.Join(sysfs, controllerClass, entry.Name())

		if readAttr(dir, "transport") != transportPCIe {
			continue
		}

		c, err := read(sysfs, dir, entry.Name())
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", entry.Name(), err))

			continue
		}

		controllers = append(controllers, c)
	}

	return controllers, errors.Join(errs...)
}

// InstanceIDs returns the fwupd-compatible instance IDs of the controller.
func (c *Controller) InstanceIDs() []string {
	return guid.NVMeInstanceIDs(c.VendorID, c.DeviceID, c.SubsystemVendorID, c.SubsystemDeviceID, c.Model)
}

// GUIDs returns the fwupd-compatible GUIDs of the controller.
func (c *Controller) GUIDs() []string {
	return guid.GUIDs(c.InstanceIDs())
}

// VendorIDs returns the fwupd-style vendor IDs of the controller.
func (c *Controller) VendorIDs() []string {
	return []string{fmt.Sprintf("NVME:0x%04X", c.VendorID)}
}

// Device describes the controller for catalog matching.
func (c *Controller) Device() lvfs.Device {
	return lvfs.Device{
		GUIDs:     c.GUIDs(),
		Protocol:  flash.ProtocolNVMe,
		Version:   c.FirmwareVersion,
		VendorIDs: c.VendorIDs(),
	}
}

// Target describes the controller for flashing.
func (c *Controller) Target() flash.Target {
	return flash.Target{DevPath: c.DevPath}
}

func read(sysfs, dir, name string) (Controller, error) {
	c := Controller{
		Name:            name,
		DevPath:         filepath.Join(devRoot, name),
		Model:           readAttr(dir, "model"),
		Serial:          readAttr(dir, "serial"),
		FirmwareVersion: readAttr(dir, "firmware_rev"),
	}

	for _, id := range []struct {
		field *uint16
		attr  string
	}{
		{&c.VendorID, "device/vendor"},
		{&c.DeviceID, "device/device"},
		{&c.SubsystemVendorID, "device/subsystem_vendor"},
		{&c.SubsystemDeviceID, "device/subsystem_device"},
	} {
		value, err := readHex(dir, id.attr)
		if err != nil {
			return Controller{}, err
		}

		*id.field = value
	}

	c.Namespaces = namespaces(sysfs, dir, name)

	return c, nil
}

// namespaces lists the namespace block devices of a controller: those under
// the controller and, with native multipath, those under the subsystem the
// controller belongs to.
func namespaces(sysfs, dir, name string) []string {
	names := namespacesIn(dir, nil)

	subsystems, _ := os.ReadDir(filepath.Join(sysfs, subsystemClass)) //nolint:errcheck

	for _, subsystem := range subsystems {
		subsystemDir := filepath.Join(sysfs, subsystemClass, subsystem.Name())

		if _, err := os.Lstat(filepath.Join(subsystemDir, name)); err != nil {
			continue
		}

		names = namespacesIn(subsystemDir, names)
	}

	slices.Sort(names)

	return names
}

func namespacesIn(dir string, names []string) []string {
	entries, _ := os.ReadDir(dir) //nolint:errcheck

	for _, entry := range entries {
		if namespaceName.MatchString(entry.Name()) && !slices.Contains(names, entry.Name()) {
			names = append(names, entry.Name())
		}
	}

	return names
}

func readAttr(dir, attr string) string {
	data, err := os.ReadFile(filepath.Join(dir, attr))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

func readHex(dir, attr string) (uint16, error) {
	value := readAttr(dir, attr)
	if value == "" {
		return 0, fmt.Errorf("missing %s", attr)
	}

	id, err := strconv.ParseUint(strings.TrimPrefix(value, "0x"), 16, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", attr, value, err)
	}

	return uint16(id), nil
}
