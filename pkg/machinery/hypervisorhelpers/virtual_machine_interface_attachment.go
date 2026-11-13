// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineInterfaceAttachment is how a guest interface is attached to its host link.
type VirtualMachineInterfaceAttachment int

// VirtualMachineInterfaceAttachment constants.
//
//structprotogen:gen_enum
const (
	VirtualMachineInterfaceAttachmentUnknown VirtualMachineInterfaceAttachment = iota // unknown
	VirtualMachineInterfaceAttachmentMacvtap                                          // macvtap
	VirtualMachineInterfaceAttachmentBridge                                           // bridge
)
