// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !amd64

package profile

import "github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"

// Profiles returns an empty catalog on unsupported architectures.
func Profiles() []*hypervisor.VirtualMachineProfileSpec {
	return nil
}

func validMachine(string) bool {
	return false
}
