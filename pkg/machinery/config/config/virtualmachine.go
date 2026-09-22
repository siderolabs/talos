// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/siderolabs/gen/optional"

// VirtualMachineConfig defines the interface to access virtual machine configuration.
type VirtualMachineConfig interface {
	NamedDocument

	// Marker for findMatchingDocs[T]
	VirtualMachineConfigSignal()

	// CPU settings; never nil.
	CPU() VirtualMachineCPUConfig
	// Memory settings; never nil.
	Memory() VirtualMachineMemoryConfig
	// Disks attached to the virtual machine, in declaration order.
	Disks() []VirtualMachineDiskConfig
	// Console settings.
	Console() VirtualMachineConsoleConfig
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

// VirtualMachineDiskFormat is the on-disk format of a virtual machine disk.
type VirtualMachineDiskFormat string

// Virtual machine disk formats.
const (
	// VirtualMachineDiskFormatRaw is a plain image with no container format. Faster on
	// block-backed pools, but has no backing chains and no snapshots.
	VirtualMachineDiskFormatRaw VirtualMachineDiskFormat = "raw"
	// VirtualMachineDiskFormatQCOW2 is the QEMU copy-on-write format. Required for a disk
	// provisioned as a linked clone. This is the default.
	VirtualMachineDiskFormatQCOW2 VirtualMachineDiskFormat = "qcow2"
)

// VirtualMachineDiskBus is the controller a disk is attached to.
type VirtualMachineDiskBus string

// Virtual machine disk buses.
const (
	// VirtualMachineDiskBusVirtio is the paravirtualized bus. This is the default, and the right
	// answer for any guest with virtio drivers.
	VirtualMachineDiskBusVirtio VirtualMachineDiskBus = "virtio"
	// VirtualMachineDiskBusSCSI is an emulated SCSI controller.
	VirtualMachineDiskBusSCSI VirtualMachineDiskBus = "scsi"
	// VirtualMachineDiskBusSATA is an emulated SATA controller, for guests without virtio drivers
	// at install time.
	VirtualMachineDiskBusSATA VirtualMachineDiskBus = "sata"
	// VirtualMachineDiskBusNVMe is an emulated NVMe controller.
	VirtualMachineDiskBusNVMe VirtualMachineDiskBus = "nvme"
)

// VirtualMachineDiskType is the kind of device a disk is presented as.
type VirtualMachineDiskType string

// Virtual machine disk types.
const (
	// VirtualMachineDiskTypeDisk is a read-write block device. This is the default.
	VirtualMachineDiskTypeDisk VirtualMachineDiskType = "disk"
	// VirtualMachineDiskTypeCDROM is a read-only optical device. QEMU emulates no CD burner, so a
	// cdrom is always read-only and its contents are always supplied by an image.
	VirtualMachineDiskTypeCDROM VirtualMachineDiskType = "cdrom"
)

// VirtualMachineDiskImageMode is how a disk's contents are derived from a content library image.
type VirtualMachineDiskImageMode string

// Virtual machine disk image modes.
const (
	// VirtualMachineDiskImageModeCopy makes a full, independent copy of the image. This is the
	// default.
	VirtualMachineDiskImageModeCopy VirtualMachineDiskImageMode = "copy"
	// VirtualMachineDiskImageModeLinked makes a thin qcow2 backed by the library image: fast and
	// space-cheap, but it pins that image for the lifetime of the disk.
	VirtualMachineDiskImageModeLinked VirtualMachineDiskImageMode = "linked"
)

// VirtualMachineDiskConfig defines a single disk attached to a virtual machine.
type VirtualMachineDiskConfig interface {
	// Name of the disk, unique within the virtual machine.
	Name() string
	// Pool is the name of the StoragePoolConfig document this disk's volume lives in.
	Pool() string
	// Size of the volume in bytes; zero for a cdrom.
	Size() uint64
	// Format of the volume, with the default applied.
	Format() VirtualMachineDiskFormat
	// Bus the disk is attached to, with the default applied.
	Bus() VirtualMachineDiskBus
	// Type of device the disk is presented as, with the default applied.
	Type() VirtualMachineDiskType
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
	Mode() VirtualMachineDiskImageMode
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
