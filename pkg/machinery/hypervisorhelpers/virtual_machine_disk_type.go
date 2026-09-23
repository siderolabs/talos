// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineDiskType is the kind of device a disk is presented as.
type VirtualMachineDiskType int

// VirtualMachineDiskType constants.
//
//structprotogen:gen_enum
const (
	VirtualMachineDiskTypeUnknown VirtualMachineDiskType = iota // unknown
	VirtualMachineDiskTypeDisk                                  // disk
	VirtualMachineDiskTypeCDROM                                 // cdrom
)
