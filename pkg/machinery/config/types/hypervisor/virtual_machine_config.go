// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// VirtualMachineConfigKind is a config document kind.
const VirtualMachineConfigKind = "VirtualMachineConfig"

func init() {
	registry.Register(VirtualMachineConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &VirtualMachineConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.VirtualMachineConfig           = &VirtualMachineConfigV1Alpha1{}
	_ config.NamedDocument                  = &VirtualMachineConfigV1Alpha1{}
	_ config.Validator                      = &VirtualMachineConfigV1Alpha1{}
	_ config.VirtualMachineCPUConfig        = &VirtualMachineCPU{}
	_ config.VirtualMachineMemoryConfig     = &VirtualMachineMemory{}
	_ config.VirtualMachineBallooningConfig = &VirtualMachineBallooning{}
)

// VirtualMachineConfigV1Alpha1 is a virtual machine configuration document.
//
//	description: |
//	  VirtualMachineConfig declares a virtual machine run by Talos.
//
//	  This document is the skeleton of the virtual machine: the rest of the machine's shape
//	  (firmware, disks, network interfaces, guest seeding, consoles) is added to it over time,
//	  and every addition is a new field rather than a change to an existing one.
//
//	  Status is reported via `VirtualMachineStatus`.
//	examples:
//	  - value: exampleVirtualMachineConfigV1Alpha1()
//	alias: VirtualMachineConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/VirtualMachineConfig
type VirtualMachineConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the virtual machine.
	//
	//     Must be between 1 and 63 characters long, and can only contain ASCII letters,
	//     digits and hyphens. It is the ID used to address the virtual machine over the API.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Power state the virtual machine is driven towards.
	//   values:
	//     - running
	//     - stopped
	//     - suspended
	//   schemaRequired: true
	PowerStateConfig hypervisorhelpers.PowerState `yaml:"powerState"`
	//   description: |
	//     Processor settings for the virtual machine.
	//   schemaRequired: true
	CPUConfig VirtualMachineCPU `yaml:"cpu"`
	//   description: |
	//     Memory settings for the virtual machine.
	//   schemaRequired: true
	MemoryConfig VirtualMachineMemory `yaml:"memory"`
	//   description: |
	//     Firmware the virtual machine boots.
	//   schemaRequired: true
	FirmwareConfig VirtualMachineFirmware `yaml:"firmware"`
	//   description: |
	//     Disks attached to the virtual machine.
	//
	//     Removing a disk detaches it from the virtual machine; the volume backing it stays in
	//     its storage pool and is deleted separately.
	//
	//     A configuration patch merges into this list by disk name: a patch entry naming an
	//     existing disk updates that disk, and any other entry is appended. Removing a disk
	//     requires supplying the document in full.
	DisksConfig VirtualMachineDiskList `yaml:"disks,omitempty"`
	//   description: |
	//     Consoles attached to the virtual machine.
	//
	//     Optional; omitting it leaves both consoles detached.
	ConsoleConfig VirtualMachineConsole `yaml:"console,omitempty"`
	//   description: |
	//     Networking settings for the virtual machine.
	//
	//     Optional; a virtual machine with no interfaces has no network connectivity at all.
	NetworkingConfig VirtualMachineNetworking `yaml:"networking,omitempty"`
}

// VirtualMachineCPU describes the processors presented to the guest.
type VirtualMachineCPU struct {
	//   description: |
	//     Number of virtual CPUs presented to the guest.
	//
	//     This is the total vCPU count, not a per-socket or per-core figure: how those vCPUs are
	//     laid out into sockets, cores and threads is not configurable.
	//   examples:
	//     - value: 4
	//   schemaRequired: true
	CPUCount uint32 `yaml:"count"`
}

// VirtualMachineMemory describes the memory presented to the guest.
type VirtualMachineMemory struct {
	//   description: |
	//     Memory allocated to the guest at boot.
	//
	//     Size is specified in bytes, but can be expressed in human readable format, e.g. 4GiB.
	//   examples:
	//     - value: >
	//         "4GiB"
	//   schema:
	//     type: string
	//   schemaRequired: true
	MemorySize meta.ByteSize `yaml:"size"`
	//   description: |
	//     Memory ballooning settings.
	//
	//     Optional; ballooning is disabled when this section is omitted.
	BallooningConfig *VirtualMachineBallooning `yaml:"ballooning,omitempty"`
}

// VirtualMachineBallooning describes the virtio-balloon settings for a virtual machine.
type VirtualMachineBallooning struct {
	//   description: |
	//     Attach a virtio-balloon device, letting the host reclaim memory the guest is not using.
	//
	//     Ballooning only shrinks the guest below `memory.size`; growing beyond it is memory
	//     hot-add, which is a separate mechanism.
	//
	//     Optional; defaults to disabled.
	BallooningEnabled *bool `yaml:"enabled,omitempty"`
}

// NewVirtualMachineConfigV1Alpha1 creates a new virtual machine config document.
func NewVirtualMachineConfigV1Alpha1() *VirtualMachineConfigV1Alpha1 {
	return &VirtualMachineConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       VirtualMachineConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleVirtualMachineConfigV1Alpha1() *VirtualMachineConfigV1Alpha1 {
	cfg := NewVirtualMachineConfigV1Alpha1()
	cfg.MetaName = "vm1"
	cfg.CPUConfig = VirtualMachineCPU{
		CPUCount: 4,
	}
	cfg.MemoryConfig = VirtualMachineMemory{
		MemorySize: meta.MustByteSize("4GiB"),
		BallooningConfig: &VirtualMachineBallooning{
			BallooningEnabled: new(true),
		},
	}
	cfg.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	cfg.FirmwareConfig = VirtualMachineFirmware{
		FirmwareType: hypervisorhelpers.VirtualMachineFirmwareTypeUEFI,
		SecureBootConfig: VirtualMachineFirmwareSecureBoot{
			SecureBootEnabled: new(true),
		},
	}
	cfg.DisksConfig = []VirtualMachineDisk{
		{
			DiskName:      "system",
			DiskPool:      "pool1",
			DiskSize:      meta.MustByteSize("20GiB"),
			DiskBootOrder: 1,
			ProvisionConfig: VirtualMachineDiskProvision{
				FromImageConfig: &VirtualMachineDiskFromImage{
					ImageLibrary: "images",
					ImageFile:    "talos-1.14.qcow2",
					ImageMode:    hypervisorhelpers.VirtualMachineDiskImageModeLinked,
				},
			},
		},
		{
			DiskName: "data",
			DiskPool: "pool1",
			DiskSize: meta.MustByteSize("100GiB"),
			ProvisionConfig: VirtualMachineDiskProvision{
				BlankConfig: &VirtualMachineDiskBlank{},
			},
		},
		{
			DiskName:      "install",
			DiskPool:      "pool1",
			DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskBootOrder: 2,
			ProvisionConfig: VirtualMachineDiskProvision{
				FromImageConfig: &VirtualMachineDiskFromImage{
					ImageLibrary: "images",
					ImageFile:    "ubuntu-24.04.iso",
				},
			},
		},
	}
	cfg.NetworkingConfig = VirtualMachineNetworking{
		InterfacesConfig: []VirtualMachineInterface{
			{
				InterfaceName: "net0",
				InterfaceLink: "eth0",
			},
			{
				InterfaceName: "net1",
				InterfaceLink: "eth1",
			},
		},
	}
	cfg.ConsoleConfig = VirtualMachineConsole{
		SerialConfig: VirtualMachineSerial{
			SerialEnabled: new(true),
		},
		VNCConfig: VirtualMachineVNC{
			VNCEnabled: new(true),
		},
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (c *VirtualMachineConfigV1Alpha1) Name() string {
	return c.MetaName
}

// Clone implements config.Document interface.
func (c *VirtualMachineConfigV1Alpha1) Clone() config.Document {
	return c.DeepCopy()
}

// VirtualMachineConfigSignal is a signal for virtual machine config.
func (c *VirtualMachineConfigV1Alpha1) VirtualMachineConfigSignal() {}

// CPU implements config.VirtualMachineConfig interface.
func (c *VirtualMachineConfigV1Alpha1) CPU() config.VirtualMachineCPUConfig {
	return &c.CPUConfig
}

// Memory implements config.VirtualMachineConfig interface.
func (c *VirtualMachineConfigV1Alpha1) Memory() config.VirtualMachineMemoryConfig {
	return &c.MemoryConfig
}

// Count implements config.VirtualMachineCPUConfig interface.
func (c *VirtualMachineCPU) Count() uint32 {
	return c.CPUCount
}

// Size implements config.VirtualMachineMemoryConfig interface.
func (m *VirtualMachineMemory) Size() uint64 {
	return m.MemorySize.Value()
}

// Ballooning implements config.VirtualMachineMemoryConfig interface.
func (m *VirtualMachineMemory) Ballooning() config.VirtualMachineBallooningConfig {
	if m.BallooningConfig == nil {
		return &VirtualMachineBallooning{}
	}

	return m.BallooningConfig
}

// Enabled implements config.VirtualMachineBallooningConfig interface.
func (b *VirtualMachineBallooning) Enabled() bool {
	return pointer.SafeDeref(b.BallooningEnabled)
}

// Disks implements config.VirtualMachineConfig interface.
func (c *VirtualMachineConfigV1Alpha1) Disks() []config.VirtualMachineDiskConfig {
	out := make([]config.VirtualMachineDiskConfig, 0, len(c.DisksConfig))

	for i := range c.DisksConfig {
		out = append(out, &c.DisksConfig[i])
	}

	return out
}

// Console implements config.VirtualMachineConfig interface.
func (c *VirtualMachineConfigV1Alpha1) Console() config.VirtualMachineConsoleConfig {
	return &c.ConsoleConfig
}

// Firmware implements config.VirtualMachineConfig interface.
func (c *VirtualMachineConfigV1Alpha1) Firmware() config.VirtualMachineFirmwareConfig {
	return &c.FirmwareConfig
}

// PowerState implements config.VirtualMachineConfig interface.
func (c *VirtualMachineConfigV1Alpha1) PowerState() hypervisorhelpers.PowerState {
	return c.PowerStateConfig
}

// Networking implements config.VirtualMachineConfig interface.
func (c *VirtualMachineConfigV1Alpha1) Networking() config.VirtualMachineNetworkingConfig {
	return &c.NetworkingConfig
}

// Validate implements config.Validator interface.
func (c *VirtualMachineConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	validationErrors = errors.Join(validationErrors, c.ValidateName())
	validationErrors = errors.Join(validationErrors, c.ValidatePowerState())
	validationErrors = errors.Join(validationErrors, c.ValidateCPU())
	validationErrors = errors.Join(validationErrors, c.ValidateMemory())
	validationErrors = errors.Join(validationErrors, c.ValidateFirmware())
	validationErrors = errors.Join(validationErrors, c.ValidateDisks())
	validationErrors = errors.Join(validationErrors, c.ValidateNetworking())

	return nil, validationErrors
}

// ValidateName checks the virtual machine name.
func (c *VirtualMachineConfigV1Alpha1) ValidateName() error {
	return validateName(c.MetaName)
}

// ValidateCPU checks the processor settings.
func (c *VirtualMachineConfigV1Alpha1) ValidateCPU() error {
	if c.CPUConfig.CPUCount == 0 {
		return errors.New("cpu.count is required")
	}

	return nil
}

// ValidateMemory checks the memory settings.
func (c *VirtualMachineConfigV1Alpha1) ValidateMemory() error {
	switch {
	case c.MemoryConfig.MemorySize.IsNegative():
		return errors.New("memory.size must not be negative")
	case c.MemoryConfig.MemorySize.IsZero():
		return errors.New("memory.size is required")
	case c.MemoryConfig.MemorySize.Value() == 0:
		return errors.New("memory.size must be greater than zero")
	}

	return nil
}

// ValidateDisks checks the disks attached to the virtual machine.
func (c *VirtualMachineConfigV1Alpha1) ValidateDisks() error {
	var validationErrors error

	names := map[string]struct{}{}
	bootOrders := map[uint32]struct{}{}

	for i := range c.DisksConfig {
		disk := &c.DisksConfig[i]

		name, err := disk.Validate(i)
		if err != nil {
			validationErrors = errors.Join(validationErrors, err)
		}

		if _, exists := names[name]; exists {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("disks[%d]: duplicate disk name %q", i, name))
		} else {
			names[name] = struct{}{}
		}

		if disk.DiskBootOrder == 0 {
			continue
		}

		if _, exists := bootOrders[disk.DiskBootOrder]; exists {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("disks[%d]: duplicate bootOrder %d", i, disk.DiskBootOrder))
		}

		bootOrders[disk.DiskBootOrder] = struct{}{}
	}

	return validationErrors
}

// ValidateNetworking checks the network interfaces of the virtual machine.
func (c *VirtualMachineConfigV1Alpha1) ValidateNetworking() error {
	var validationErrors error

	names := map[string]struct{}{}

	for i := range c.NetworkingConfig.InterfacesConfig {
		iface := &c.NetworkingConfig.InterfacesConfig[i]

		// The uniqueness check below runs whatever the interface's own validation said: an
		// interface which fails a rule of its own still occupies its name, and skipping it would
		// hide a collision until the other error is fixed.
		name, err := iface.Validate(i)
		validationErrors = errors.Join(validationErrors, err)

		// An empty name is reported by the interface itself, so it is not also collided on.
		if name != "" {
			if _, exists := names[name]; exists {
				validationErrors = errors.Join(validationErrors,
					fmt.Errorf("networking.interfaces[%d]: duplicate interface name %q", i, name))
			}

			names[name] = struct{}{}
		}
	}

	return validationErrors
}

// ValidateFirmware checks the firmware the virtual machine boots.
func (c *VirtualMachineConfigV1Alpha1) ValidateFirmware() error {
	return c.FirmwareConfig.validate()
}

// ValidatePowerState checks the power state the virtual machine is driven towards.
//
// Unmarshalling rejects any name outside the enum, so the only cases left are a field that was
// never set and a value built in Go rather than decoded.
func (c *VirtualMachineConfigV1Alpha1) ValidatePowerState() error {
	switch {
	case c.PowerStateConfig == hypervisorhelpers.PowerStateUnknown:
		return errors.New("powerState is required")
	case !c.PowerStateConfig.IsAPowerState():
		return fmt.Errorf("unsupported powerState %q", c.PowerStateConfig)
	}

	return nil
}
