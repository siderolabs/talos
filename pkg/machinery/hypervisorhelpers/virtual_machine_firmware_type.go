// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineFirmwareType is the firmware a virtual machine boots.
type VirtualMachineFirmwareType int

// VirtualMachineFirmwareType constants.
//
//structprotogen:gen_enum
const (
	VirtualMachineFirmwareTypeUnknown VirtualMachineFirmwareType = iota // unknown
	VirtualMachineFirmwareTypeUEFI                                      // uefi
	VirtualMachineFirmwareTypeBIOS                                      // bios
)
