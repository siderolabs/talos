// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import "runtime"

const accelerator = "kvm"

func (arch Arch) acceleratorAvailable() bool {
	if err := checkKVM(); err != nil {
		return false
	}

	// kvm only supports emulating native architectures
	return string(arch) == runtime.GOARCH
}
