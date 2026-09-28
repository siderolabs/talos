// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineNUMAMode is how guest memory is bound to the host NUMA nodes it is placed on.
type VirtualMachineNUMAMode int

// VirtualMachineNUMAMode constants.
//
//structprotogen:gen_enum
const (
	VirtualMachineNUMAModeUnknown    VirtualMachineNUMAMode = iota // unknown
	VirtualMachineNUMAModeStrict                                   // strict
	VirtualMachineNUMAModePreferred                                // preferred
	VirtualMachineNUMAModeInterleave                               // interleave
)
