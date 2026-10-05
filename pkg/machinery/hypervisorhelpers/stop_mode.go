// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// StopMode is the way a virtual machine is taken down when it is driven towards the stopped power
// state.
//
// It is a parameter of a transition rather than a desired state, so it is not something a machine
// configuration declares: it is carried by the VirtualMachineStopMode resource the hypervisor API
// writes. The zero member stands for a stop nothing asked anything of, which destroys the domain.
type StopMode int

// StopMode constants.
//
//structprotogen:gen_enum
const (
	StopModeUnknown  StopMode = iota // unknown
	StopModeGraceful                 // graceful
	StopModeForced                   // forced
)
