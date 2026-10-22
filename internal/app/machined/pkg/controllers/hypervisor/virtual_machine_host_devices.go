// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// defaultSysfsPath is where the host's sysfs is mounted.
const defaultSysfsPath = "/sys"

// vfioIOMMUBackendModule is the VFIO IOMMU backend libvirt's VFIO containers rely on. It is not
// pulled in by vfio_pci itself: the kernel only declares it a soft dependency of vfio, which
// Talos's module loading does not follow.
const vfioIOMMUBackendModule = "vfio_iommu_type1"

// errHostDevicePending marks a start waiting on a host device to be handed to vfio-pci, e.g. by a
// PCIDriverRebindConfig which has not been applied yet.
var errHostDevicePending = errors.New("host device is not bound to " + hypervisorhelpers.VFIOPCIDriver)

// errHostDeviceUnavailable marks a host device this host cannot pass through as it stands: it is
// absent, or the host IOMMU is disabled. Unlike errHostDevicePending, a rebind does not fix it.
var errHostDeviceUnavailable = errors.New("host device is unavailable")

// errVFIOBackendPending marks a start waiting on the VFIO IOMMU backend module to be loaded.
var errVFIOBackendPending = errors.New("VFIO IOMMU backend is not available")

// errHostDeviceClaimed marks a start waiting on a host device, or a device in its IOMMU group,
// which another virtual machine passes through: VFIO opens a group for one user at a time.
var errHostDeviceClaimed = errors.New("host device is claimed by another virtual machine")

// isHostDeviceHeld reports a start held back by its host devices rather than failed.
func isHostDeviceHeld(err error) bool {
	return errors.Is(err, errHostDevicePending) || errors.Is(err, errHostDeviceUnavailable) ||
		errors.Is(err, errVFIOBackendPending) || errors.Is(err, errHostDeviceClaimed)
}

// vfioGroupDrivers are the drivers a device may be bound to while another device of its IOMMU
// group is passed through: VFIO only opens a group whose every device is bound to one of them, or
// to no driver at all.
var vfioGroupDrivers = []string{hypervisorhelpers.VFIOPCIDriver, "pci-stub"}

// pciClaim lists the PCI devices another virtual machine passes through.
type pciClaim struct {
	name      string
	addresses []string
	// running is set while the virtual machine's domain runs: it holds its devices.
	running bool
}

// listPCIClaims returns the PCI devices the other virtual machines pass through: those meant to run,
// and those whose domain still runs. A virtual machine whose own domain runs already holds its
// devices, so nothing is claimed against it.
func listPCIClaims(ctx context.Context, r controller.Reader, name string) ([]pciClaim, error) {
	running, err := runningDomains(ctx, r)
	if err != nil {
		return nil, err
	}

	if running[name] {
		return nil, nil
	}

	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("list virtual machine specs: %w", err)
	}

	var claims []pciClaim

	for spec := range specs.All() {
		if claim, ok := specPCIClaim(spec, name, running); ok {
			claims = append(claims, claim)
		}
	}

	return claims, nil
}

// runningDomains returns which virtual machines have a running domain.
func runningDomains(ctx context.Context, r controller.Reader) (map[string]bool, error) {
	domains, err := safe.ReaderListAll[*hypervisor.VirtualMachineDomainStatus](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("list virtual machine domain statuses: %w", err)
	}

	running := make(map[string]bool, domains.Len())

	for domain := range domains.All() {
		running[domain.Metadata().ID()] = domain.TypedSpec().PowerState == hypervisorhelpers.VirtualMachinePowerStateRunning
	}

	return running, nil
}

// specPCIClaim returns the claim of another virtual machine on its PCI devices, if it is meant to
// run or its domain still runs.
func specPCIClaim(spec *hypervisor.VirtualMachineSpec, name string, running map[string]bool) (pciClaim, bool) {
	other := spec.Metadata().ID()
	if other == name || len(spec.TypedSpec().PCIDevices) == 0 {
		return pciClaim{}, false
	}

	claim := pciClaim{name: other, running: running[other]}

	if !claim.running && spec.TypedSpec().PowerState != hypervisorhelpers.VirtualMachinePowerStateRunning.String() {
		return pciClaim{}, false
	}

	for _, device := range spec.TypedSpec().PCIDevices {
		if address, err := hypervisorhelpers.ParsePCIAddress(device.Address); err == nil {
			claim.addresses = append(claim.addresses, address.String())
		}
	}

	return claim, true
}

// checkPCIClaims holds a start back while another virtual machine passes through one of its
// devices, or a device in the same IOMMU group. A running virtual machine keeps its devices; between
// two which are not running, the one whose name sorts first gets them, so exactly one starts.
func checkPCIClaims(devicesPath, name string, addresses []string, claims []pciClaim) error {
	for _, address := range addresses {
		group := iommuGroup(devicesPath, address)

		for _, claim := range claims {
			if !claim.running && claim.name > name {
				continue
			}

			for _, claimed := range claim.addresses {
				switch {
				case claimed == address:
					return fmt.Errorf("PCI device %q: %w %q", address, errHostDeviceClaimed, claim.name)
				case group != "" && iommuGroup(devicesPath, claimed) == group:
					return fmt.Errorf("PCI device %q: %w %q, which passes through %q in the same IOMMU group %s",
						address, errHostDeviceClaimed, claim.name, claimed, group)
				}
			}
		}
	}

	return nil
}

// iommuGroup returns the IOMMU group of a PCI device, or an empty string when it has none.
func iommuGroup(devicesPath, address string) string {
	group, err := os.Readlink(filepath.Join(devicesPath, address, "iommu_group"))
	if err != nil {
		return ""
	}

	return filepath.Base(group)
}

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
		if hostdev.SubsysPCI == nil || hostdev.SubsysPCI.Source == nil || hostdev.SubsysPCI.Source.Address == nil {
			continue
		}

		source := hostdev.SubsysPCI.Source.Address
		if source.Domain == nil || source.Bus == nil || source.Slot == nil || source.Function == nil {
			continue
		}

		addresses = append(addresses, hypervisorhelpers.PCIAddress{
			Domain:   uint16(*source.Domain),
			Bus:      uint8(*source.Bus),
			Slot:     uint8(*source.Slot),
			Function: uint8(*source.Function),
		}.String())
	}

	return addresses
}

// checkDomainHostDevices checks that every PCI device the exact domain definition passes through
// is present, sits behind an enabled IOMMU, and is bound to vfio-pci with the rest of its IOMMU
// group viable, that no other virtual machine claims it or its group, and that the VFIO IOMMU
// backend is loaded.
//
// The host is read directly rather than through the PCIDevice resources, which are a snapshot taken
// at boot and do not follow a later rebind. An empty sysfs path reads the host's own sysfs.
func checkDomainHostDevices(sysfsPath, name, domainXML string, claims []pciClaim) error {
	if sysfsPath == "" {
		sysfsPath = defaultSysfsPath
	}

	var domain libvirtxml.Domain

	if err := domain.Unmarshal(domainXML); err != nil {
		return fmt.Errorf("virtual machine %q: decode domain: %w", name, err)
	}

	addresses := domainPCIDevices(&domain)
	if len(addresses) == 0 {
		return nil
	}

	devicesPath := filepath.Join(sysfsPath, "bus", "pci", "devices")

	for _, address := range addresses {
		if err := checkPCIDevice(devicesPath, address); err != nil {
			return fmt.Errorf("virtual machine %q: PCI device %q: %w", name, address, err)
		}
	}

	if err := checkPCIClaims(devicesPath, name, addresses, claims); err != nil {
		return fmt.Errorf("virtual machine %q: %w", name, err)
	}

	// Without the backend, QEMU fails to open the device's IOMMU group ("No available IOMMU
	// models") and exits as the domain starts.
	if _, err := os.Stat(filepath.Join(sysfsPath, "module", vfioIOMMUBackendModule)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("virtual machine %q: %w: ensure the %s kernel module is loaded", name, errVFIOBackendPending, vfioIOMMUBackendModule)
		}

		return err
	}

	return nil
}

func checkPCIDevice(root, address string) error {
	device := filepath.Join(root, address)

	if _, err := os.Stat(device); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: not present on the host", errHostDeviceUnavailable)
		}

		return err
	}

	if _, err := os.Stat(filepath.Join(device, "iommu_group")); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: device has no IOMMU group, ensure the IOMMU is enabled on the host", errHostDeviceUnavailable)
		}

		return err
	}

	driver, err := os.Readlink(filepath.Join(device, "driver"))

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: device has no driver, ensure the vfio_pci kernel module is loaded", errHostDevicePending)
	case err != nil:
		return err
	case filepath.Base(driver) != hypervisorhelpers.VFIOPCIDriver:
		return fmt.Errorf("%w: device is bound to %q, add a PCIDriverRebindConfig targeting %s",
			errHostDevicePending, filepath.Base(driver), hypervisorhelpers.VFIOPCIDriver)
	}

	return checkIOMMUGroupViable(root, address)
}

// checkIOMMUGroupViable checks that every other device in the IOMMU group of a device is bound to
// vfio-pci, pci-stub or no driver: VFIO refuses to open a group with a device still on a host
// driver, and QEMU would only find out as the domain starts.
func checkIOMMUGroupViable(root, address string) error {
	members, err := os.ReadDir(filepath.Join(root, address, "iommu_group", "devices"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return err
	}

	for _, member := range members {
		if member.Name() == address {
			continue
		}

		driver, err := os.Readlink(filepath.Join(root, address, "iommu_group", "devices", member.Name(), "driver"))

		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return err
		case slices.Contains(vfioGroupDrivers, filepath.Base(driver)):
			continue
		}

		return fmt.Errorf("%w: device %q in the same IOMMU group %s is bound to %q; every device in the group needs vfio-pci, pci-stub or no driver",
			errHostDevicePending, member.Name(), iommuGroup(root, address), filepath.Base(driver))
	}

	return nil
}
