// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineDiskFormat is the on-disk format of a virtual machine disk.
type VirtualMachineDiskFormat int

// VirtualMachineDiskFormat constants.
//
//structprotogen:gen_enum
const (
	VirtualMachineDiskFormatUnknown VirtualMachineDiskFormat = iota // unknown
	VirtualMachineDiskFormatRaw                                     // raw
	VirtualMachineDiskFormatQCOW2                                   // qcow2
)
