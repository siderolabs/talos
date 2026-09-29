// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// PowerState is the power state a virtual machine is driven towards.
type PowerState int

// PowerState constants.
//
//structprotogen:gen_enum
const (
	PowerStateUnknown   PowerState = iota // unknown
	PowerStateRunning                     // running
	PowerStateStopped                     // stopped
	PowerStateSuspended                   // suspended
)
