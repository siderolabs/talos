// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hypervisorhelpers provides types and type wrappers to support hypervisor resources.
package hypervisorhelpers

//go:generate go tool github.com/dmarkham/enumer -type=PowerState,VirtualMachineDiskBus,VirtualMachineDiskFormat,VirtualMachineDiskImageMode,VirtualMachineDiskType,VirtualMachineFirmwareType -linecomment -text
