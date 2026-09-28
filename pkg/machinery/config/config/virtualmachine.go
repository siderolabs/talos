// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// VirtualMachineConfig defines the interface to access virtual machine configuration.
type VirtualMachineConfig interface {
	NamedDocument

	// Marker for findMatchingDocs[T]
	VirtualMachineConfigSignal()

	// PowerState the virtual machine is driven towards.
	PowerState() hypervisorhelpers.PowerState
	// CPU settings; never nil.
	CPU() VirtualMachineCPUConfig
	// Memory settings; never nil.
	Memory() VirtualMachineMemoryConfig
	// Firmware settings.
	Firmware() VirtualMachineFirmwareConfig
	// Disks attached to the virtual machine, in declaration order.
	Disks() []VirtualMachineDiskConfig
	// Console settings.
	Console() VirtualMachineConsoleConfig
	// Networking settings.
	Networking() VirtualMachineNetworkingConfig
	// Guest settings; never nil, and zero-valued when the guest section is omitted.
	Guest() VirtualMachineGuestConfig
}

// VirtualMachineCPUConfig defines the processors presented to the guest.
type VirtualMachineCPUConfig interface {
	// Count is the total number of vCPUs.
	Count() uint32
}

// VirtualMachineMemoryConfig defines the memory presented to the guest.
type VirtualMachineMemoryConfig interface {
	// Size is the memory allocated to the guest at boot, in bytes.
	Size() uint64
	// Ballooning settings; never nil.
	Ballooning() VirtualMachineBallooningConfig
}

// VirtualMachineBallooningConfig defines the virtio-balloon settings for a virtual machine.
//
//nolint:iface
type VirtualMachineBallooningConfig interface {
	// Enabled reports whether a virtio-balloon device is attached, with the default applied.
	Enabled() bool
}

// VirtualMachineDiskConfig defines a single disk attached to a virtual machine.
type VirtualMachineDiskConfig interface {
	// Name of the disk, unique within the virtual machine.
	Name() string
	// Pool is the name of the StoragePoolConfig document this disk's volume lives in.
	Pool() string
	// Size of the volume in bytes; zero for a cdrom.
	Size() uint64
	// Format of the volume, with the default applied.
	Format() hypervisorhelpers.VirtualMachineDiskFormat
	// Bus the disk is attached to, with the default applied.
	Bus() hypervisorhelpers.VirtualMachineDiskBus
	// Type of device the disk is presented as, with the default applied.
	Type() hypervisorhelpers.VirtualMachineDiskType
	// BootOrder of the disk; zero means the disk is not in the boot order.
	BootOrder() uint32
	// Provision describes where the volume's contents come from; never nil.
	Provision() VirtualMachineDiskProvisionConfig
}

// VirtualMachineDiskProvisionConfig defines where a disk's contents come from.
//
// Exactly one of the sources is present.
type VirtualMachineDiskProvisionConfig interface {
	// Blank reports whether the volume is created empty.
	Blank() bool
	// FromImage describes the content library image the volume is derived from.
	FromImage() optional.Optional[VirtualMachineDiskFromImageConfig]
}

// VirtualMachineDiskFromImageConfig derives a disk from a content library image.
type VirtualMachineDiskFromImageConfig interface {
	// Library is the name of the ContentLibraryConfig document holding the image.
	Library() string
	// File is the name of the image within that library.
	File() string
	// Digest is an optional integrity check of the library file, as `sha256:<hex>`.
	Digest() string
	// Mode is how the volume is derived from the image, with the default applied.
	Mode() hypervisorhelpers.VirtualMachineDiskImageMode
}

// VirtualMachineConsoleConfig defines the consoles attached to a virtual machine.
type VirtualMachineConsoleConfig interface {
	// Serial console settings
	Serial() VirtualMachineSerialConfig
	// VNC console settings
	VNC() VirtualMachineVNCConfig
}

// VirtualMachineSerialConfig defines the serial console of a virtual machine.
//
//nolint:iface
type VirtualMachineSerialConfig interface {
	// Enabled reports whether the serial console should be attached.
	Enabled() bool
}

// VirtualMachineVNCConfig defines the VNC console of a virtual machine.
//
//nolint:iface
type VirtualMachineVNCConfig interface {
	// Enabled reports whether the VNC console should be attached.
	Enabled() bool
}

// VirtualMachineFirmwareConfig defines the firmware a virtual machine boots.
type VirtualMachineFirmwareConfig interface {
	// Type of firmware the guest boots.
	Type() hypervisorhelpers.VirtualMachineFirmwareType
	// SecureBoot settings.
	SecureBoot() VirtualMachineFirmwareSecureBootConfig
}

// VirtualMachineFirmwareSecureBootConfig defines the secure boot settings of a virtual machine.
//
//nolint:iface
type VirtualMachineFirmwareSecureBootConfig interface {
	// Enabled reports whether the guest boots with secure boot, with the default applied.
	Enabled() bool
}

// VirtualMachineNetworkingConfig defines the networking of a virtual machine.
//
//nolint:iface
type VirtualMachineNetworkingConfig interface {
	// Interfaces attached to the virtual machine, in declaration order.
	Interfaces() []VirtualMachineInterfaceConfig
}

// VirtualMachineInterfaceConfig defines a single network interface of a virtual machine.
type VirtualMachineInterfaceConfig interface {
	// Name of the interface, unique within the virtual machine.
	Name() string
	// Link is the kernel name of the host link the interface is attached to.
	Link() string
}

// VirtualMachineGuestConfig defines the settings which apply inside the guest.
//
//nolint:iface
type VirtualMachineGuestConfig interface {
	// CloudInit is the seed handed to the guest on first boot.
	CloudInit() optional.Optional[VirtualMachineCloudInitConfig]
	// Agent settings; never nil.
	Agent() VirtualMachineAgentConfig
}

// VirtualMachineAgentConfig defines the qemu-guest-agent settings for a virtual machine.
//
//nolint:iface
type VirtualMachineAgentConfig interface {
	// Enabled reports whether the guest agent channel is attached, with the default applied.
	Enabled() bool
}

// VirtualMachineCloudInitConfig defines the NoCloud seed handed to the guest.
//
// The three values are the three files of a NoCloud seed, passed through verbatim: Talos renders
// them into the seed and does not interpret them.
type VirtualMachineCloudInitConfig interface {
	// MetaData is the `meta-data` file, carrying the guest's identity.
	MetaData() string
	// UserData is the `user-data` file, carrying what the operator wants done.
	UserData() string
	// NetworkConfig is the `network-config` file, carrying the guest's network settings.
	NetworkConfig() string
}
