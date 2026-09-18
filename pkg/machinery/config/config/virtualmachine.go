// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// VirtualMachineConfig defines the interface to access virtual machine configuration.
type VirtualMachineConfig interface {
	NamedDocument

	// Marker for findMatchingDocs[T]
	VirtualMachineConfigSignal()

	// CPU settings; never nil.
	CPU() VirtualMachineCPUConfig
	// Memory settings; never nil.
	Memory() VirtualMachineMemoryConfig
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
