// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineDiskBus is the controller a disk is attached to.
type VirtualMachineDiskBus int

// VirtualMachineDiskBus constants.
//
//structprotogen:gen_enum
const (
	VirtualMachineDiskBusUnknown VirtualMachineDiskBus = iota // unknown
	VirtualMachineDiskBusVirtio                               // virtio
	VirtualMachineDiskBusSCSI                                 // scsi
	VirtualMachineDiskBusSATA                                 // sata
	VirtualMachineDiskBusNVMe                                 // nvme
)
