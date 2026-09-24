// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hypervisorhelpers provides types and type wrappers to support hypervisor resources.
package hypervisorhelpers

const (
	// CPUQuotaPeriod is the whole-domain CFS enforcement interval in microseconds.
	CPUQuotaPeriod = 100000

	// MinCPULimitMillicores corresponds to libvirt's minimum cpuquota of 1000 microseconds.
	MinCPULimitMillicores = 1000 * 1000 / CPUQuotaPeriod

	// MaxCPULimitMillicores keeps the quota within libvirt's maximum of 17592186044415 microseconds.
	MaxCPULimitMillicores = 17592186044415 * 1000 / CPUQuotaPeriod
)

//go:generate go tool github.com/dmarkham/enumer -type=PowerState,VirtualMachineDiskBus,VirtualMachineDiskFormat,VirtualMachineDiskImageMode,VirtualMachineDiskType,VirtualMachineFirmwareType -linecomment -text
