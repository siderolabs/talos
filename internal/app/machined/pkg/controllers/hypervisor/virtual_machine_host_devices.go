// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// defaultSysfsPath is where the host's sysfs is mounted.
const defaultSysfsPath = "/sys"

// errHostDeviceNotReady marks a start waiting on a passed-through PCI device to be present and
// bound to vfio-pci, e.g. by a PCIDriverRebindConfig which has not been applied yet.
var errHostDeviceNotReady = errors.New("host device is not ready")

// validateVirtualMachinePCIDevices checks the passed-through PCI devices, and that their boot
// orders do not collide with each other or with the disks'.
func validateVirtualMachinePCIDevices(name string, spec *hypervisor.VirtualMachineSpecSpec) error {
	bootOrders := make(map[uint32]struct{}, len(spec.Disks))

	for _, disk := range spec.Disks {
		if disk.BootOrder != 0 {
			bootOrders[disk.BootOrder] = struct{}{}
		}
	}

	addresses := make(map[string]struct{}, len(spec.PCIDevices))

	for _, device := range spec.PCIDevices {
		address, err := hypervisorhelpers.ParsePCIAddress(device.Address)
		if err != nil {
			return fmt.Errorf("virtual machine %q: %w", name, err)
		}

		if _, exists := addresses[address.String()]; exists {
			return fmt.Errorf("virtual machine %q: duplicate PCI device %q", name, address.String())
		}

		addresses[address.String()] = struct{}{}

		if device.BootOrder == 0 {
			continue
		}

		if _, exists := bootOrders[device.BootOrder]; exists {
			return fmt.Errorf("virtual machine %q: PCI device %q: duplicate boot order %d", name, address.String(), device.BootOrder)
		}

		bootOrders[device.BootOrder] = struct{}{}
	}

	return nil
}

// renderVirtualMachinePCIDevices attaches every passed-through PCI device, in configuration order.
//
// Devices are attached unmanaged: libvirt is built without its node device driver, so Talos binds
// a device to vfio-pci itself (PCIDriverRebindConfig) and libvirt only opens it. A managed device
// would have libvirt detach it from, and reattach it to, its host driver on every start and stop.
func renderVirtualMachinePCIDevices(domain *libvirtxml.Domain, devices []hypervisor.VirtualMachinePCIDeviceSpec) error {
	for _, device := range devices {
		address, err := hypervisorhelpers.ParsePCIAddress(device.Address)
		if err != nil {
			return err
		}

		pciDomain, bus, slot, function := uint(address.Domain), uint(address.Bus), uint(address.Slot), uint(address.Function)

		hostdev := libvirtxml.DomainHostdev{
			Managed: "no",
			SubsysPCI: &libvirtxml.DomainHostdevSubsysPCI{
				Driver: &libvirtxml.DomainHostdevSubsysPCIDriver{
					Name: "vfio",
				},
				Source: &libvirtxml.DomainHostdevSubsysPCISource{
					Address: &libvirtxml.DomainAddressPCI{
						Domain:   &pciDomain,
						Bus:      &bus,
						Slot:     &slot,
						Function: &function,
					},
				},
			},
			// The user alias carries the host address onto the device, so it can be found in the
			// live definition without depending on the guest address libvirt assigns.
			Alias: &libvirtxml.DomainAlias{
				Name: pciDeviceAlias(address),
			},
		}

		if device.BootOrder != 0 {
			hostdev.Boot = &libvirtxml.DomainDeviceBoot{Order: uint(device.BootOrder)}
		}

		domain.Devices.Hostdevs = append(domain.Devices.Hostdevs, hostdev)
	}

	return nil
}

// renderVirtualMachinePassthroughClock keeps the PIT from re-injecting missed ticks in a guest with
// passed-through PCI devices.
//
// QEMU's in-kernel PIT re-injects missed ticks by default, and on AMD hosts KVM inhibits AVIC for the
// whole guest while it does. Without AVIC, interrupts of passed-through devices are not posted to the
// guest: the host handles every one of them, on whichever host CPU the vector lands. The PIT itself is
// kept, as BIOS firmware still relies on it; guests keep time with kvmclock.
func renderVirtualMachinePassthroughClock(domain *libvirtxml.Domain) {
	domain.Clock = &libvirtxml.DomainClock{
		Offset: "utc",
		Timer: []libvirtxml.DomainTimer{
			{Name: "pit", TickPolicy: "discard"},
		},
	}
}

// pciDeviceAlias is the user alias of a passed-through PCI device; aliases only allow letters,
// digits, '_' and '-'.
func pciDeviceAlias(address hypervisorhelpers.PCIAddress) string {
	return interfaceAliasPrefix + "pci-" + strings.NewReplacer(":", "-", ".", "-").Replace(address.String())
}

// domainPCIDevices collects the host addresses of the PCI devices the definition passes through.
func domainPCIDevices(domain *libvirtxml.Domain) []string {
	if domain.Devices == nil {
		return nil
	}

	var addresses []string

	for _, hostdev := range domain.Devices.Hostdevs {
		if hostdev.SubsysPCI != nil {
			addresses = appendPCISource(addresses, hostdev.SubsysPCI.Source)
		}
	}

	return addresses
}

// appendPCISource appends the host address of a PCI device source, when it has a complete one.
func appendPCISource(addresses []string, source *libvirtxml.DomainHostdevSubsysPCISource) []string {
	if source == nil || source.Address == nil {
		return addresses
	}

	address := source.Address
	if address.Domain == nil || address.Bus == nil || address.Slot == nil || address.Function == nil {
		return addresses
	}

	return append(addresses, hypervisorhelpers.PCIAddress{
		Domain:   uint16(*address.Domain),
		Bus:      uint8(*address.Bus),
		Slot:     uint8(*address.Slot),
		Function: uint8(*address.Function),
	}.String())
}

// checkDomainHostDevices checks that every PCI device the exact domain definition passes through
// is present and bound to vfio-pci: libvirt only opens an unmanaged device, so a start with one on
// another driver would fail.
//
// The host is read directly rather than through the PCIDevice resources, which are a snapshot taken
// at boot and do not follow a later rebind. An empty sysfs path reads the host's own sysfs.
func checkDomainHostDevices(sysfsPath, name, domainXML string) error {
	if sysfsPath == "" {
		sysfsPath = defaultSysfsPath
	}

	var domain libvirtxml.Domain

	if err := domain.Unmarshal(domainXML); err != nil {
		return fmt.Errorf("virtual machine %q: decode domain: %w", name, err)
	}

	for _, address := range domainPCIDevices(&domain) {
		if err := checkPCIDevice(filepath.Join(sysfsPath, "bus", "pci", "devices", address)); err != nil {
			return fmt.Errorf("virtual machine %q: PCI device %q: %w", name, address, err)
		}
	}

	return nil
}

func checkPCIDevice(device string) error {
	if _, err := os.Stat(device); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: not present on the host", errHostDeviceNotReady)
		}

		return err
	}

	driver, err := os.Readlink(filepath.Join(device, "driver"))

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: device has no driver, ensure the vfio_pci kernel module is loaded", errHostDeviceNotReady)
	case err != nil:
		return err
	case filepath.Base(driver) != hypervisorhelpers.VFIOPCIDriver:
		return fmt.Errorf("%w: device is bound to %q, add a PCIDriverRebindConfig targeting %s",
			errHostDeviceNotReady, filepath.Base(driver), hypervisorhelpers.VFIOPCIDriver)
	}

	return nil
}
