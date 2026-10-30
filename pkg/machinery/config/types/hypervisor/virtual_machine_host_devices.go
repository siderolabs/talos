// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// Check interfaces.
var (
	_ config.VirtualMachineHostDevicesConfig = &VirtualMachineHostDevices{}
	_ config.VirtualMachinePCIDeviceConfig   = &VirtualMachinePCIDevice{}
	_ yaml.IsZeroer                          = VirtualMachineHostDevices{}
)

// VirtualMachineHostDevices describes the host devices passed through to a virtual machine.
type VirtualMachineHostDevices struct {
	//   description: |
	//     Host PCI devices passed through to the guest.
	//
	//     Each device must be bound to the `vfio-pci` driver on the host before the virtual
	//     machine starts, e.g. with a `PCIDriverRebindConfig` document, and the `vfio_pci` and
	//     `vfio_iommu_type1` kernel modules must be loaded (`KernelModuleConfig`). Until then the
	//     virtual machine is held back and its status reports what it is waiting for.
	//
	//     Every other device in the same IOMMU group must be bound to `vfio-pci`, `pci-stub` or
	//     no driver for the group to be usable; passing through only some functions of a
	//     multi-function device usually still requires the others to be detached from their host
	//     drivers. A device, or its IOMMU group, can be passed through to one virtual machine at a
	//     time.
	//
	//     Each device gets a guest PCI slot of its own: functions which must share a slot in the
	//     guest, such as a GPU and its audio function, are not supported yet.
	//
	//     A passed-through device pins all of the guest memory on the host, so memory ballooning
	//     has no effect on such a guest.
	//
	//     A configuration patch replaces this list as a whole rather than appending to it.
	PCIConfig []VirtualMachinePCIDevice `yaml:"pci,omitempty" merge:"replace"`
}

// VirtualMachinePCIDevice describes a single host PCI device passed through to a virtual machine.
type VirtualMachinePCIDevice struct {
	//   description: |
	//     Address of the device on the host, in the full form `domain:bus:slot.function`.
	//
	//     This is the device name under `/sys/bus/pci/devices` and the ID reported by
	//     `talosctl get pcidevices`.
	//   examples:
	//     - value: '"0000:41:00.0"'
	//   schemaRequired: true
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-1][0-9a-fA-F]\.[0-7]$
	PCIAddress string `yaml:"address"`
	//   description: |
	//     Position of the device in the guest boot order, shared with the disks of the virtual
	//     machine; e.g. to boot from a passed-through NVMe drive or a network card's option ROM.
	//
	//     Optional; zero leaves the device out of the boot order.
	PCIBootOrder uint32 `yaml:"bootOrder,omitempty"`
}

// IsZero implements yaml.IsZeroer.
func (d VirtualMachineHostDevices) IsZero() bool {
	return len(d.PCIConfig) == 0
}

// PCI implements config.VirtualMachineHostDevicesConfig interface.
func (d *VirtualMachineHostDevices) PCI() []config.VirtualMachinePCIDeviceConfig {
	out := make([]config.VirtualMachinePCIDeviceConfig, 0, len(d.PCIConfig))

	for i := range d.PCIConfig {
		out = append(out, &d.PCIConfig[i])
	}

	return out
}

// Address implements config.VirtualMachinePCIDeviceConfig interface.
//
// The address is returned in its canonical form; an address which fails to parse is returned as
// written, as validation has already rejected it.
func (p *VirtualMachinePCIDevice) Address() string {
	address, err := hypervisorhelpers.ParsePCIAddress(p.PCIAddress)
	if err != nil {
		return p.PCIAddress
	}

	return address.String()
}

// BootOrder implements config.VirtualMachinePCIDeviceConfig interface.
func (p *VirtualMachinePCIDevice) BootOrder() uint32 {
	return p.PCIBootOrder
}

// validate checks the host devices, against each other and against the boot orders the disks
// already claim.
func (d *VirtualMachineHostDevices) validate(diskBootOrders map[uint32]struct{}) error {
	var validationErrors error

	addresses := map[string]struct{}{}

	for i := range d.PCIConfig {
		device := &d.PCIConfig[i]

		address, err := hypervisorhelpers.ParsePCIAddress(device.PCIAddress)
		if err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("hostDevices.pci[%d]: %w", i, err))

			continue
		}

		// Compared in canonical form, so the same device written in two cases still collides.
		if _, exists := addresses[address.String()]; exists {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("hostDevices.pci[%d]: duplicate PCI address %q", i, address.String()))
		}

		addresses[address.String()] = struct{}{}

		if device.PCIBootOrder == 0 {
			continue
		}

		if _, exists := diskBootOrders[device.PCIBootOrder]; exists {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("hostDevices.pci[%d]: duplicate bootOrder %d", i, device.PCIBootOrder))
		}

		diskBootOrders[device.PCIBootOrder] = struct{}{}
	}

	return validationErrors
}
