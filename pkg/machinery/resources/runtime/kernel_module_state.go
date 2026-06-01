// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import "fmt"

// KernelModuleState represents the operational state of a kernel module.
type KernelModuleState int

// KernelModuleState constants.
//
//structprotogen:gen_enum
const (
	KernelModuleStateLive      KernelModuleState = iota // live
	KernelModuleStateLoading                            // loading
	KernelModuleStateUnloading                          // unloading
	KernelModuleStateBuiltin                            // built-in
)

// ParseDynamicModuleState converts a string representation of a kernel module state into
// its corresponding KernelModuleState constant.
// 'Builtin' is intentionally omitted as it is not a valid state for dynamically loaded modules.
func ParseDynamicModuleState(s string) (KernelModuleState, error) {
	switch s {
	case "Live":
		return KernelModuleStateLive, nil
	case "Loading":
		return KernelModuleStateLoading, nil
	case "Unloading":
		return KernelModuleStateUnloading, nil
	default:
		return 0, fmt.Errorf("unknown kernel module state %q", s)
	}
}
