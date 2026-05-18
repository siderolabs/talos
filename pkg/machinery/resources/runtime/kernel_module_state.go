// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

// KernelModuleState represents the operational state of a dynamically loaded kernel module.
type KernelModuleState int

// KernelModuleState constants.
//
//structprotogen:gen_enum
const (
	KernelModuleStateInactive  KernelModuleState = iota // inactive
	KernelModuleStateActive                             // active
	KernelModuleStateLoading                            // loading
	KernelModuleStateUnloading                          // unloading
)

// ParseDynamicModuleState converts a string representation of a kernel module state into
// its corresponding KernelModuleState constant.
func ParseDynamicModuleState(s string) KernelModuleState {
	var state KernelModuleState

	switch s {
	case "Live":
		state = KernelModuleStateActive
	case "Loading":
		state = KernelModuleStateLoading
	case "Unloading":
		state = KernelModuleStateUnloading
	default:
		state = KernelModuleStateInactive
	}

	return state
}
