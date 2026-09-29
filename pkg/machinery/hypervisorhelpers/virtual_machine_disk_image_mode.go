// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineDiskImageMode is how a disk's contents are derived from a content library image.
type VirtualMachineDiskImageMode int

// VirtualMachineDiskImageMode constants.
//
//structprotogen:gen_enum
const (
	VirtualMachineDiskImageModeUnknown VirtualMachineDiskImageMode = iota // unknown
	VirtualMachineDiskImageModeCopy                                       // copy
	VirtualMachineDiskImageModeLinked                                     // linked
)
