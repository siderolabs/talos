// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

// KernelModuleType represents whether a kernel module is built into the kernel or dynamically loaded.
type KernelModuleType int

// KernelModuleType constants.
//
//structprotogen:gen_enum
const (
	KernelModuleTypeUnknown KernelModuleType = iota // unknown
	KernelModuleTypeBuiltin                         // built-in
	KernelModuleTypeDynamic                         // dynamic
)
